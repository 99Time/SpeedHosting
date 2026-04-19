package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"speedhosting/backend/internal/models"
)

var (
	ErrValidation             = errors.New("validation failed")
	ErrInvalidCredentials     = errors.New("invalid email or password")
	ErrEmailAlreadyRegistered = errors.New("email address is already registered")
	ErrInvalidSession         = errors.New("invalid session")
)

type Service struct {
	db         *sql.DB
	sessionTTL time.Duration
}

type RegisterInput struct {
	DisplayName string
	Email       string
	Password    string
	Attribution models.AcquisitionAttribution
}

type LoginInput struct {
	Email       string
	Password    string
	Attribution models.AcquisitionAttribution
}

type SessionMetadata struct {
	UserAgent string
	IPAddress string
}

func NewService(db *sql.DB, sessionTTL time.Duration) *Service {
	return &Service{db: db, sessionTTL: sessionTTL}
}

func (s *Service) Register(ctx context.Context, input RegisterInput, metadata SessionMetadata) (models.AuthenticatedUser, string, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = normalizeEmail(input.Email)

	if err := validateRegistrationInput(input); err != nil {
		return models.AuthenticatedUser{}, "", err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return models.AuthenticatedUser{}, "", fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.AuthenticatedUser{}, "", fmt.Errorf("begin register tx: %w", err)
	}
	defer tx.Rollback()

	planID, err := s.lookupFreePlanID(ctx, tx)
	if err != nil {
		return models.AuthenticatedUser{}, "", err
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO users (email, password_hash, display_name, plan_id, role)
		VALUES (?, ?, ?, ?, ?)`, input.Email, string(passwordHash), input.DisplayName, planID, s.registrationRole(ctx, tx))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "users.email") {
			return models.AuthenticatedUser{}, "", ErrEmailAlreadyRegistered
		}

		return models.AuthenticatedUser{}, "", fmt.Errorf("insert user: %w", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return models.AuthenticatedUser{}, "", fmt.Errorf("read user id: %w", err)
	}

	user, err := s.lookupUserByID(ctx, tx, userID)
	if err != nil {
		return models.AuthenticatedUser{}, "", err
	}

	if err := s.persistAttribution(ctx, tx, user.ID, input.Attribution); err != nil {
		return models.AuthenticatedUser{}, "", err
	}

	user, err = s.lookupUserByID(ctx, tx, userID)
	if err != nil {
		return models.AuthenticatedUser{}, "", err
	}

	token, err := s.createSession(ctx, tx, user.ID, metadata)
	if err != nil {
		return models.AuthenticatedUser{}, "", err
	}

	if err := tx.Commit(); err != nil {
		return models.AuthenticatedUser{}, "", fmt.Errorf("commit register tx: %w", err)
	}

	return user, token, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput, metadata SessionMetadata) (models.AuthenticatedUser, string, error) {
	input.Email = normalizeEmail(input.Email)
	if input.Email == "" || strings.TrimSpace(input.Password) == "" {
		return models.AuthenticatedUser{}, "", ErrInvalidCredentials
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.AuthenticatedUser{}, "", fmt.Errorf("begin login tx: %w", err)
	}
	defer tx.Rollback()

	var user models.AuthenticatedUser
	var passwordHash string
	err = tx.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.display_name, p.code, u.role, u.password_hash
		FROM users u
		JOIN plans p ON p.id = u.plan_id
		WHERE u.email = ?`, input.Email).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PlanCode, &user.Role, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.AuthenticatedUser{}, "", ErrInvalidCredentials
		}

		return models.AuthenticatedUser{}, "", fmt.Errorf("query user for login: %w", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
		return models.AuthenticatedUser{}, "", ErrInvalidCredentials
	}

	if err := s.persistAttribution(ctx, tx, user.ID, input.Attribution); err != nil {
		return models.AuthenticatedUser{}, "", err
	}

	user, err = s.lookupUserByID(ctx, tx, user.ID)
	if err != nil {
		return models.AuthenticatedUser{}, "", err
	}

	token, err := s.createSession(ctx, tx, user.ID, metadata)
	if err != nil {
		return models.AuthenticatedUser{}, "", err
	}

	if err := tx.Commit(); err != nil {
		return models.AuthenticatedUser{}, "", fmt.Errorf("commit login tx: %w", err)
	}

	return user, token, nil
}

func (s *Service) AuthenticateSession(ctx context.Context, rawToken string) (models.AuthenticatedUser, error) {
	tokenHash := hashToken(rawToken)
	if tokenHash == "" {
		return models.AuthenticatedUser{}, ErrInvalidSession
	}

	var user models.AuthenticatedUser
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.display_name, p.code, u.role,
		       COALESCE(u.first_acquisition_source, ''), COALESCE(u.latest_acquisition_source, ''), COALESCE(u.first_acquisition_timestamp, '')
		FROM auth_sessions sessions
		JOIN users u ON u.id = sessions.user_id
		JOIN plans p ON p.id = u.plan_id
		WHERE sessions.token_hash = ?
		  AND datetime(sessions.expires_at) > datetime('now')`, tokenHash).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PlanCode, &user.Role, &user.FirstAcquisitionSource, &user.LatestAcquisitionSource, &user.FirstAcquisitionTimestamp)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.AuthenticatedUser{}, ErrInvalidSession
		}

		return models.AuthenticatedUser{}, fmt.Errorf("query session: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE auth_sessions
		SET last_seen_at = CURRENT_TIMESTAMP
		WHERE token_hash = ?`, tokenHash); err != nil {
		return models.AuthenticatedUser{}, fmt.Errorf("touch session: %w", err)
	}

	return user, nil
}

func (s *Service) DeleteSession(ctx context.Context, rawToken string) error {
	tokenHash := hashToken(rawToken)
	if tokenHash == "" {
		return nil
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func (s *Service) lookupFreePlanID(ctx context.Context, tx *sql.Tx) (int64, error) {
	var planID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM plans WHERE code = 'free'`).Scan(&planID)
	if err != nil {
		return 0, fmt.Errorf("lookup free plan: %w", err)
	}

	return planID, nil
}

func (s *Service) lookupUserByID(ctx context.Context, tx *sql.Tx, userID int64) (models.AuthenticatedUser, error) {
	var user models.AuthenticatedUser
	err := tx.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.display_name, p.code, u.role,
		       COALESCE(u.first_acquisition_source, ''), COALESCE(u.latest_acquisition_source, ''), COALESCE(u.first_acquisition_timestamp, '')
		FROM users u
		JOIN plans p ON p.id = u.plan_id
		WHERE u.id = ?`, userID).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PlanCode, &user.Role, &user.FirstAcquisitionSource, &user.LatestAcquisitionSource, &user.FirstAcquisitionTimestamp)
	if err != nil {
		return models.AuthenticatedUser{}, fmt.Errorf("load registered user: %w", err)
	}

	return user, nil
}

func (s *Service) persistAttribution(ctx context.Context, tx *sql.Tx, userID int64, attribution models.AcquisitionAttribution) error {
	source := sanitizeAttributionSource(attribution.Source)
	if source == "" {
		return nil
	}

	timestamp := normalizeAttributionTimestamp(attribution.Timestamp)
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET first_acquisition_source = COALESCE(first_acquisition_source, ?),
		    latest_acquisition_source = ?,
		    first_acquisition_timestamp = COALESCE(first_acquisition_timestamp, ?),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, source, source, timestamp, userID); err != nil {
		return fmt.Errorf("persist user attribution: %w", err)
	}

	return nil
}

func sanitizeAttributionSource(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	buffer := strings.Builder{}
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
			buffer.WriteRune(character)
		case character >= '0' && character <= '9':
			buffer.WriteRune(character)
		case character == '-' || character == '_':
			buffer.WriteRune(character)
		}
	}

	return buffer.String()
}

func normalizeAttributionTimestamp(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().UTC().Format(time.RFC3339)
	}

	return value
}

func (s *Service) registrationRole(ctx context.Context, tx *sql.Tx) string {
	var adminCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&adminCount); err != nil {
		return "customer"
	}

	if adminCount == 0 {
		return "admin"
	}

	return "customer"
}

func (s *Service) createSession(ctx context.Context, tx *sql.Tx, userID int64, metadata SessionMetadata) (string, error) {
	rawToken, err := generateToken()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().UTC().Add(s.sessionTTL).Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO auth_sessions (user_id, token_hash, expires_at, user_agent, ip_address)
		VALUES (?, ?, ?, ?, ?)`, userID, hashToken(rawToken), expiresAt, sanitizeNullable(metadata.UserAgent), sanitizeNullable(metadata.IPAddress)); err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}

	return rawToken, nil
}

func generateToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashToken(rawToken string) string {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func validateRegistrationInput(input RegisterInput) error {
	if len(input.DisplayName) < 2 || len(input.DisplayName) > 60 {
		return fmt.Errorf("%w: display name must be between 2 and 60 characters", ErrValidation)
	}

	if _, err := mail.ParseAddress(input.Email); err != nil {
		return fmt.Errorf("%w: email address is invalid", ErrValidation)
	}

	if len(input.Password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrValidation)
	}

	return nil
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func sanitizeNullable(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return value
}
