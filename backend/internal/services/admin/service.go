package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"speedhosting/backend/internal/models"
	"speedhosting/backend/internal/planrules"
	analyticsservice "speedhosting/backend/internal/services/analytics"
	serverservice "speedhosting/backend/internal/services/server"
)

var ErrPlanNotFound = errors.New("plan not found")

type Service struct {
	db        *sql.DB
	analytics *analyticsservice.Service
}

func NewService(db *sql.DB, analytics *analyticsservice.Service) *Service {
	return &Service{db: db, analytics: analytics}
}

func (s *Service) Overview(ctx context.Context) (models.AdminOverview, error) {
	plans, err := s.loadPlans(ctx)
	if err != nil {
		return models.AdminOverview{}, err
	}

	users, err := s.loadUsers(ctx)
	if err != nil {
		return models.AdminOverview{}, err
	}

	servers, err := s.loadServers(ctx)
	if err != nil {
		return models.AdminOverview{}, err
	}

	attributionSummary, err := s.loadAttributionSummary(ctx)
	if err != nil {
		return models.AdminOverview{}, err
	}

	return models.AdminOverview{Users: users, Servers: servers, Plans: plans, AttributionSummary: attributionSummary}, nil
}

func (s *Service) UpdateUserPlan(ctx context.Context, userID int64, planCode string) (models.AdminUserSummary, error) {
	previousPlanCode, err := s.currentPlanCode(ctx, userID)
	if err != nil {
		return models.AdminUserSummary{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET plan_id = (SELECT id FROM plans WHERE code = ?), updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND EXISTS (SELECT 1 FROM plans WHERE code = ?)`, planCode, userID, planCode)
	if err != nil {
		return models.AdminUserSummary{}, fmt.Errorf("update user plan: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.AdminUserSummary{}, fmt.Errorf("user plan rows affected: %w", err)
	}

	if rowsAffected == 0 {
		var userExists int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ?`, userID).Scan(&userExists); err != nil {
			return models.AdminUserSummary{}, fmt.Errorf("check user existence: %w", err)
		}

		if userExists == 0 {
			return models.AdminUserSummary{}, sql.ErrNoRows
		}

		return models.AdminUserSummary{}, ErrPlanNotFound
	}

	user, err := s.loadUserByID(ctx, userID)
	if err != nil {
		return models.AdminUserSummary{}, err
	}

	if previousPlanCode != "pro" && user.PlanCode == "pro" && s.analytics != nil {
		_ = s.analytics.TrackProUpgradeSuccess(ctx, userID)
	}

	return user, nil
}

func (s *Service) loadPlans(ctx context.Context) ([]models.Plan, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code, name, max_servers, max_tick_rate, max_admins, allow_custom_mods, allow_advanced_config
		FROM plans
		ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("load plans: %w", err)
	}
	defer rows.Close()

	plans := make([]models.Plan, 0)
	for rows.Next() {
		var plan models.Plan
		var allowCustomMods int
		var allowAdvancedConfig int
		if err := rows.Scan(
			&plan.ID,
			&plan.Code,
			&plan.Name,
			&plan.MaxServers,
			&plan.MaxTickRate,
			&plan.MaxAdmins,
			&allowCustomMods,
			&allowAdvancedConfig,
		); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}

		plan.AllowCustomMods = allowCustomMods == 1
		plan.AllowAdvancedConfig = allowAdvancedConfig == 1
		plan = planrules.Apply(plan)
		plans = append(plans, plan)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plans: %w", err)
	}

	return plans, nil
}

func (s *Service) loadUsers(ctx context.Context) ([]models.AdminUserSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.email, u.display_name, p.code, u.role, COUNT(s.id),
		       COALESCE(u.first_acquisition_source, ''), COALESCE(u.latest_acquisition_source, '')
		FROM users u
		JOIN plans p ON p.id = u.plan_id
		LEFT JOIN servers s ON s.owner_id = u.id
		GROUP BY u.id, u.email, u.display_name, p.code, u.role
		ORDER BY CASE WHEN u.role = 'admin' THEN 0 ELSE 1 END, u.created_at ASC, u.id ASC`)
	if err != nil {
		return nil, fmt.Errorf("load users: %w", err)
	}
	defer rows.Close()

	users := make([]models.AdminUserSummary, 0)
	for rows.Next() {
		var user models.AdminUserSummary
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.PlanCode, &user.Role, &user.ServerCount, &user.FirstAcquisitionSource, &user.LatestAcquisitionSource); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

func (s *Service) loadServers(ctx context.Context) ([]models.AdminServerSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.owner_id, s.name, u.email, u.display_name, p.code, s.region, s.status, s.config_file_path, s.service_name,
		       s.desired_tick_rate, COALESCE(r.player_count, 0), s.max_players, COALESCE(r.process_state, s.status), s.desired_tick_rate, COALESCE(r.last_action_error, ''), s.config_json
		FROM servers s
		JOIN users u ON u.id = s.owner_id
		JOIN plans p ON p.id = u.plan_id
		LEFT JOIN server_runtime r ON r.server_id = s.id
		ORDER BY s.created_at DESC, s.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("load servers: %w", err)
	}
	defer rows.Close()

	servers := make([]models.AdminServerSummary, 0)
	for rows.Next() {
		var server models.AdminServerSummary
		var rawConfigJSON string
		if err := rows.Scan(
			&server.ID,
			&server.OwnerID,
			&server.Name,
			&server.OwnerEmail,
			&server.OwnerName,
			&server.PlanCode,
			&server.Region,
			&server.Status,
			&server.ConfigFilePath,
			&server.ServiceName,
			&server.DesiredTickRate,
			&server.PlayerCount,
			&server.MaxPlayers,
			&server.ProcessState,
			&server.TickRate,
			&server.LastActionError,
			&rawConfigJSON,
		); err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}

		server.Config, err = serverservice.ParseStoredConfig(rawConfigJSON)
		if err != nil {
			return nil, fmt.Errorf("parse admin server config: %w", err)
		}
		if server.TickRate <= 0 {
			server.TickRate = server.Config.ServerTickRate
		}
		servers = append(servers, server)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate servers: %w", err)
	}

	return servers, nil
}

func (s *Service) loadUserByID(ctx context.Context, userID int64) (models.AdminUserSummary, error) {
	var user models.AdminUserSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.display_name, p.code, u.role, COUNT(s.id),
		       COALESCE(u.first_acquisition_source, ''), COALESCE(u.latest_acquisition_source, '')
		FROM users u
		JOIN plans p ON p.id = u.plan_id
		LEFT JOIN servers s ON s.owner_id = u.id
		WHERE u.id = ?
		GROUP BY u.id, u.email, u.display_name, p.code, u.role`, userID).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PlanCode, &user.Role, &user.ServerCount, &user.FirstAcquisitionSource, &user.LatestAcquisitionSource)
	if err != nil {
		return models.AdminUserSummary{}, fmt.Errorf("load user summary: %w", err)
	}

	return user, nil
}

func (s *Service) loadAttributionSummary(ctx context.Context) ([]models.AttributionSourceReport, error) {
	if s.analytics == nil {
		return []models.AttributionSourceReport{}, nil
	}

	return s.analytics.LoadAttributionSummary(ctx)
}

func (s *Service) currentPlanCode(ctx context.Context, userID int64) (string, error) {
	var planCode string
	err := s.db.QueryRowContext(ctx, `
		SELECT p.code
		FROM users u
		JOIN plans p ON p.id = u.plan_id
		WHERE u.id = ?`, userID).Scan(&planCode)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", sql.ErrNoRows
		}

		return "", fmt.Errorf("load current plan code: %w", err)
	}

	return planCode, nil
}
