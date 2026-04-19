package ranked

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"speedhosting/backend/internal/models"
	"speedhosting/backend/internal/ranktier"
)

var (
	ErrPlayerNotFound       = errors.New("ranked player not found")
	ErrInvalidLinkCode      = errors.New("invalid link code")
	ErrLinkCodeExpired      = errors.New("link code expired")
	ErrLinkCodeAlreadyUsed  = errors.New("link code already used")
	ErrDiscordAlreadyLinked = errors.New("discord id is already linked")
)

const (
	linkSessionPending   = "pending"
	linkSessionCompleted = "completed"
	linkSessionExpired   = "expired"
	linkSessionCancelled = "cancelled"
	defaultLinkCodeTTL   = 10 * time.Minute
)

type Service struct {
	db          *sql.DB
	logger      *log.Logger
	dbPath      string
	dataPath    string
	mmrPath     string
	starsPath   string
	linkCodeTTL time.Duration
	mu          sync.RWMutex
	cached      []models.RankedLeaderboardEntry
}

type discordLink struct {
	DiscordID                 string
	SteamID                   string
	DiscordDisplay            string
	LastKnownGameName         string
	LastKnownGamePlayerNumber string
}

type linkSession struct {
	Code        string
	DiscordID   string
	GuildID     string
	ChannelID   string
	Status      string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	CompletedAt sql.NullString
	Used        bool
	SteamID     string
}

type snapshotFile struct {
	Entries     []models.RankedLeaderboardEntry `json:"entries"`
	Players     []models.RankedLeaderboardEntry `json:"players"`
	Leaderboard []models.RankedLeaderboardEntry `json:"leaderboard"`
}

type mmrFileEntry struct {
	DisplayName string `json:"displayName"`
	MMR         int    `json:"mmr"`
	Wins        int    `json:"wins"`
	Losses      int    `json:"losses"`
}

type starsFileEntry struct {
	StarPoints int `json:"starPoints"`
	WinStreak  int `json:"winStreak"`
}

type mmrFileWrapper struct {
	Players map[string]mmrFileEntry `json:"players"`
	Data    map[string]mmrFileEntry `json:"data"`
}

type starsFileWrapper struct {
	Players map[string]starsFileEntry `json:"players"`
	Data    map[string]starsFileEntry `json:"data"`
}

type persistedRankedProfile struct {
	SteamID          string
	DisplayName      string
	MMR              int
	Goals            int
	Assists          int
	SecondaryAssists int
	Wins             int
	Losses           int
	StarPoints       int
	WinStreak        int
}

func NewService(db *sql.DB, logger *log.Logger, dbPath string, dataPath string, mmrPath string, starsPath string, linkCodeTTL time.Duration) *Service {
	if linkCodeTTL <= 0 {
		linkCodeTTL = defaultLinkCodeTTL
	}

	return &Service{
		db:          db,
		logger:      logger,
		dbPath:      strings.TrimSpace(dbPath),
		dataPath:    strings.TrimSpace(dataPath),
		mmrPath:     strings.TrimSpace(mmrPath),
		starsPath:   strings.TrimSpace(starsPath),
		linkCodeTTL: linkCodeTTL,
		cached:      defaultSnapshot(),
	}
}

func (s *Service) Leaderboard(ctx context.Context, limit int) ([]models.RankedLeaderboardEntry, error) {
	entries, err := s.loadSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > len(entries) {
		limit = len(entries)
	}

	result := append([]models.RankedLeaderboardEntry(nil), entries[:limit]...)
	enriched, err := s.enrichEntriesWithLinks(ctx, result)
	if err != nil {
		return nil, err
	}
	if len(enriched) > 0 {
		top := enriched[0]
		s.logf("[ranked] leaderboard response limit=%d entries=%d top_steam=%s top_mmr=%d top_tier=%s", limit, len(enriched), maskIdentifier(top.SteamID), top.MMR, top.Tier.TierKey)
	} else {
		s.logf("[ranked] leaderboard response limit=%d entries=0", limit)
	}

	return enriched, nil
}

func (s *Service) Rank(ctx context.Context, steamID string, discordID string, query string) (models.RankedPlayer, error) {
	entries, err := s.loadSnapshot(ctx)
	if err != nil {
		return models.RankedPlayer{}, err
	}

	steamID = strings.TrimSpace(steamID)
	discordID = strings.TrimSpace(discordID)
	query = strings.TrimSpace(query)

	if steamID == "" && discordID != "" {
		s.logf("[ranked] rank lookup start discord=%s", maskIdentifier(normalizeDigits(discordID)))
		resolvedSteamID, resolveErr := s.GetSteamByDiscord(ctx, discordID)
		if resolveErr == nil {
			steamID = resolvedSteamID
			s.logf("[ranked] rank lookup linked discord=%s steam=%s", maskIdentifier(normalizeDigits(discordID)), maskIdentifier(steamID))
		} else if errors.Is(resolveErr, sql.ErrNoRows) {
			s.logf("[ranked] rank lookup not linked discord=%s", maskIdentifier(normalizeDigits(discordID)))
		} else {
			s.logf("[ranked] rank lookup discord resolution error discord=%s err=%v", maskIdentifier(normalizeDigits(discordID)), resolveErr)
			return models.RankedPlayer{}, resolveErr
		}
	}

	enrichedEntries, err := s.enrichEntriesWithLinks(ctx, entries)
	if err != nil {
		return models.RankedPlayer{}, err
	}

	for _, entry := range enrichedEntries {
		if steamID != "" && entry.SteamID == steamID {
			s.logf("[ranked] rank lookup success steam=%s rank=%d mmr=%d tier=%s api_game_player_number=%q", maskIdentifier(steamID), entry.Rank, entry.MMR, entry.Tier.TierKey, entry.LastKnownGamePlayerNumber)
			return toRankedPlayer(entry), nil
		}
		if discordID != "" && entry.LinkedDiscordID != "" && entry.LinkedDiscordID == discordID {
			s.logf("[ranked] rank lookup success discord=%s rank=%d mmr=%d tier=%s api_game_player_number=%q", maskIdentifier(normalizeDigits(discordID)), entry.Rank, entry.MMR, entry.Tier.TierKey, entry.LastKnownGamePlayerNumber)
			return toRankedPlayer(entry), nil
		}
	}

	if query != "" {
		queryLower := strings.ToLower(query)
		for _, entry := range enrichedEntries {
			if strings.EqualFold(entry.DisplayName, query) || entry.SteamID == query || entry.LinkedDiscordID == query || strings.EqualFold(entry.LinkedDiscordDisplay, query) {
				s.logf("[ranked] rank lookup query exact query=%q rank=%d mmr=%d tier=%s api_game_player_number=%q", query, entry.Rank, entry.MMR, entry.Tier.TierKey, entry.LastKnownGamePlayerNumber)
				return toRankedPlayer(entry), nil
			}
		}
		for _, entry := range enrichedEntries {
			if strings.Contains(strings.ToLower(entry.DisplayName), queryLower) || strings.Contains(strings.ToLower(entry.LinkedDiscordDisplay), queryLower) {
				s.logf("[ranked] rank lookup query fuzzy query=%q rank=%d mmr=%d tier=%s api_game_player_number=%q", query, entry.Rank, entry.MMR, entry.Tier.TierKey, entry.LastKnownGamePlayerNumber)
				return toRankedPlayer(entry), nil
			}
		}
	}

	s.logf("[ranked] rank lookup result not found discord=%s steam=%s query=%q", maskIdentifier(normalizeDigits(discordID)), maskIdentifier(steamID), query)

	return models.RankedPlayer{}, ErrPlayerNotFound
}

func (s *Service) LinkSteam(ctx context.Context, discordID string, steamID string, discordDisplay string) error {
	discordID = normalizeDigits(discordID)
	steamID = normalizeDigits(steamID)
	discordDisplay = strings.TrimSpace(discordDisplay)

	if err := validateDiscordID(discordID); err != nil {
		return err
	}
	if err := validateSteamID(steamID); err != nil {
		return err
	}

	if discordDisplay == "" {
		discordDisplay = safeDiscordDisplay("", discordID)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin discord link tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM discord_links WHERE discord_id = ? OR steam_id = ?`, discordID, steamID); err != nil {
		return fmt.Errorf("delete existing discord link: %w", err)
	}

	currentDiscordDisplay, currentGameName, currentGamePlayerNumber, err := currentLinkPresentation(ctx, tx, discordID, steamID)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO discord_links (discord_id, steam_id, discord_display, last_known_game_name, last_known_game_player_number, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, discordID, steamID, nullableString(firstNonEmptyString(discordDisplay, currentDiscordDisplay)), nullableString(currentGameName), nullableString(currentGamePlayerNumber)); err != nil {
		return fmt.Errorf("insert discord link: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE ranked_link_sessions
		SET status = ?, used = 1
		WHERE discord_id = ? AND status = ? AND used = 0`, linkSessionCancelled, discordID, linkSessionPending); err != nil {
		return fmt.Errorf("cancel pending discord link sessions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit discord link: %w", err)
	}

	return nil
}

func (s *Service) RequestLink(ctx context.Context, discordID string, guildID string, channelID string) (models.RankedLinkRequestResult, error) {
	discordID = normalizeDigits(discordID)
	guildID = normalizeDigits(guildID)
	channelID = normalizeDigits(channelID)
	s.logf("[ranked] request link start db=%s discord=%s guild=%s channel=%s branch=start", s.dbPath, maskIdentifier(discordID), maskIdentifier(guildID), maskIdentifier(channelID))

	if err := validateDiscordID(discordID); err != nil {
		s.logf("[ranked] request link db=%s discord=%s branch=invalid_discord err=%v", s.dbPath, maskIdentifier(discordID), err)
		return models.RankedLinkRequestResult{}, err
	}

	steamID, linked, err := s.lookupPermanentLinkByDiscord(ctx, discordID)
	if err != nil {
		return models.RankedLinkRequestResult{}, err
	}
	if linked {
		if clearErr := s.cancelPendingSessionsForDiscord(ctx, discordID); clearErr != nil {
			s.logf("[ranked] request link db=%s discord=%s branch=already_linked_cancel_pending_error err=%v", s.dbPath, maskIdentifier(discordID), clearErr)
			return models.RankedLinkRequestResult{}, clearErr
		}
		s.logf("[ranked] request link db=%s discord=%s steam=%s branch=already_linked result=ok", s.dbPath, maskIdentifier(discordID), maskIdentifier(steamID))
		return models.RankedLinkRequestResult{
			OK:            true,
			AlreadyLinked: true,
			SteamID:       steamID,
		}, nil
	}

	now := time.Now().UTC()
	if err := s.expirePendingSessions(ctx, now); err != nil {
		return models.RankedLinkRequestResult{}, err
	}

	session, found, err := s.findPendingSessionByDiscord(ctx, discordID, now)
	if err != nil {
		s.logf("[ranked] request link db=%s discord=%s branch=find_pending_error err=%v", s.dbPath, maskIdentifier(discordID), err)
		return models.RankedLinkRequestResult{}, err
	}

	if found {
		if guildID != session.GuildID || channelID != session.ChannelID {
			if err := s.updatePendingSessionContext(ctx, session.Code, guildID, channelID); err != nil {
				s.logf("[ranked] request link db=%s discord=%s code=%s branch=update_pending_context_error err=%v", s.dbPath, maskIdentifier(discordID), session.Code, err)
				return models.RankedLinkRequestResult{}, err
			}
			session.GuildID = guildID
			session.ChannelID = channelID
		}

		result := models.RankedLinkRequestResult{
			OK:               true,
			Code:             session.Code,
			ExpiresInSeconds: secondsUntil(session.ExpiresAt, now),
			AlreadyOpen:      true,
		}
		s.logf("[ranked] request link db=%s discord=%s code=%s branch=reuse_pending result=ok expires_in=%ds", s.dbPath, maskIdentifier(discordID), session.Code, result.ExpiresInSeconds)
		return result, nil
	}

	createdAt := now
	expiresAt := now.Add(s.linkCodeTTL)
	for attempt := 0; attempt < 10; attempt++ {
		code, err := generateLinkCode()
		if err != nil {
			s.logf("[ranked] request link db=%s discord=%s branch=generate_code_error err=%v", s.dbPath, maskIdentifier(discordID), err)
			return models.RankedLinkRequestResult{}, err
		}

		insertResult, err := s.db.ExecContext(ctx, `
			INSERT INTO ranked_link_sessions (code, discord_id, guild_id, channel_id, status, created_at, expires_at, used)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0)`,
			code,
			discordID,
			nullableString(guildID),
			nullableString(channelID),
			linkSessionPending,
			createdAt.Format(time.RFC3339),
			expiresAt.Format(time.RFC3339),
		)
		if err == nil {
			rowsAffected, rowsErr := insertResult.RowsAffected()
			if rowsErr != nil {
				s.logf("[ranked] request link db=%s discord=%s code=%s branch=insert_pending_rows_error err=%v", s.dbPath, maskIdentifier(discordID), code, rowsErr)
				return models.RankedLinkRequestResult{}, fmt.Errorf("ranked link session insert rows affected: %w", rowsErr)
			}
			if rowsAffected <= 0 {
				err = fmt.Errorf("ranked link session insert produced no rows")
				s.logf("[ranked] request link db=%s discord=%s code=%s branch=insert_pending_zero_rows err=%v", s.dbPath, maskIdentifier(discordID), code, err)
				return models.RankedLinkRequestResult{}, err
			}

			persistedCount, countErr := s.countLinkSessionsByCode(ctx, code)
			if countErr != nil {
				s.logf("[ranked] request link db=%s discord=%s code=%s branch=verify_pending_error err=%v", s.dbPath, maskIdentifier(discordID), code, countErr)
				return models.RankedLinkRequestResult{}, countErr
			}
			if persistedCount <= 0 {
				err = fmt.Errorf("ranked link session was not persisted")
				s.logf("[ranked] request link db=%s discord=%s code=%s branch=verify_pending_missing err=%v", s.dbPath, maskIdentifier(discordID), code, err)
				return models.RankedLinkRequestResult{}, err
			}

			s.logf("[ranked] created pending link session discord=%s code=%s expires_at=%s", maskIdentifier(discordID), maskCode(code), expiresAt.Format(time.RFC3339))
			s.logf("[ranked] request link db=%s discord=%s code=%s branch=insert_pending result=ok rows=%d persisted_count=%d expires_at=%s", s.dbPath, maskIdentifier(discordID), code, rowsAffected, persistedCount, expiresAt.Format(time.RFC3339))
			return models.RankedLinkRequestResult{
				OK:               true,
				Code:             code,
				ExpiresInSeconds: secondsUntil(expiresAt, now),
				AlreadyOpen:      false,
			}, nil
		}

		if isPendingDiscordConflict(err) {
			existingSession, exists, lookupErr := s.findPendingSessionByDiscord(ctx, discordID, now)
			if lookupErr != nil {
				s.logf("[ranked] request link db=%s discord=%s code=%s branch=conflict_lookup_error err=%v", s.dbPath, maskIdentifier(discordID), code, lookupErr)
				return models.RankedLinkRequestResult{}, lookupErr
			}
			if exists {
				result := models.RankedLinkRequestResult{
					OK:               true,
					Code:             existingSession.Code,
					ExpiresInSeconds: secondsUntil(existingSession.ExpiresAt, now),
					AlreadyOpen:      true,
				}
				s.logf("[ranked] request link db=%s discord=%s code=%s branch=reuse_after_conflict result=ok expires_in=%ds", s.dbPath, maskIdentifier(discordID), existingSession.Code, result.ExpiresInSeconds)
				return result, nil
			}
		}

		if !isUniqueCodeError(err) {
			s.logf("[ranked] request link db=%s discord=%s code=%s branch=insert_pending_error err=%v", s.dbPath, maskIdentifier(discordID), code, err)
			return models.RankedLinkRequestResult{}, fmt.Errorf("create ranked link session: %w", err)
		}
	}

	s.logf("[ranked] request link db=%s discord=%s branch=code_allocation_failed", s.dbPath, maskIdentifier(discordID))
	return models.RankedLinkRequestResult{}, fmt.Errorf("unable to allocate a unique ranked link code")
}

func (s *Service) LinkStatus(ctx context.Context, discordID string) (models.RankedLinkStatus, error) {
	discordID = normalizeDigits(discordID)
	if err := validateDiscordID(discordID); err != nil {
		return models.RankedLinkStatus{}, err
	}

	now := time.Now().UTC()
	if err := s.expirePendingSessions(ctx, now); err != nil {
		return models.RankedLinkStatus{}, err
	}

	steamID, linked, err := s.lookupPermanentLinkByDiscord(ctx, discordID)
	if err != nil {
		return models.RankedLinkStatus{}, err
	}
	if linked {
		if clearErr := s.cancelPendingSessionsForDiscord(ctx, discordID); clearErr != nil {
			return models.RankedLinkStatus{}, clearErr
		}
		s.logf("[ranked] link status linked discord=%s steam=%s", maskIdentifier(discordID), maskIdentifier(steamID))
		return models.RankedLinkStatus{Linked: true, SteamID: steamID, Pending: false}, nil
	}

	session, found, err := s.findPendingSessionByDiscord(ctx, discordID, now)
	if err != nil {
		return models.RankedLinkStatus{}, err
	}
	if found {
		s.logf("[ranked] link status pending discord=%s code=%s expires_in=%ds", maskIdentifier(discordID), maskCode(session.Code), secondsUntil(session.ExpiresAt, now))
		return models.RankedLinkStatus{
			Linked:           false,
			Pending:          true,
			Code:             session.Code,
			ExpiresInSeconds: secondsUntil(session.ExpiresAt, now),
			ChannelID:        session.ChannelID,
		}, nil
	}

	s.logf("[ranked] link status empty discord=%s", maskIdentifier(discordID))

	return models.RankedLinkStatus{Linked: false, Pending: false}, nil
}

func (s *Service) CompleteLink(ctx context.Context, steamID string, code string, gameDisplayName string, rawGamePlayerNumber string, gamePlayerNumber string) (models.RankedLinkCompleteResult, error) {
	steamID = normalizeDigits(steamID)
	code = normalizeLinkCode(code)
	gameDisplayName = sanitizeGameDisplayName(gameDisplayName)
	rawGamePlayerNumber = strings.TrimSpace(rawGamePlayerNumber)
	gamePlayerNumber = sanitizeGamePlayerNumber(gamePlayerNumber)
	s.logf("[ranked] complete link start db=%s steam=%s code=%s branch=start raw_game_player_number=%q normalized_game_player_number=%q", s.dbPath, maskIdentifier(steamID), code, rawGamePlayerNumber, gamePlayerNumber)

	if err := validateSteamID(steamID); err != nil {
		s.logf("[ranked] complete link db=%s steam=%s code=%s branch=invalid_steam err=%v", s.dbPath, maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, err
	}
	if code == "" {
		s.logf("[ranked] complete link db=%s steam=%s branch=empty_code", s.dbPath, maskIdentifier(steamID))
		return models.RankedLinkCompleteResult{}, ErrInvalidLinkCode
	}

	existingRankedPlayer, rankedProfileExists, existingRankedErr := s.findRankedPlayerBySteamID(ctx, steamID)
	if existingRankedErr != nil {
		s.logf("[ranked] complete link db=%s steam=%s code=%s branch=existing_rank_lookup_error err=%v", s.dbPath, maskIdentifier(steamID), code, existingRankedErr)
		return models.RankedLinkCompleteResult{}, existingRankedErr
	}
	if rankedProfileExists {
		s.logf("[ranked] complete link db=%s steam=%s code=%s branch=existing_rank_lookup result=found mmr=%d tier=%s", s.dbPath, maskIdentifier(steamID), code, existingRankedPlayer.MMR, existingRankedPlayer.Tier.TierKey)
	} else {
		s.logf("[ranked] complete link db=%s steam=%s code=%s branch=existing_rank_lookup result=missing", s.dbPath, maskIdentifier(steamID), code)
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logf("[ranked] complete link db=%s steam=%s code=%s branch=begin_tx_error err=%v", s.dbPath, maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, fmt.Errorf("begin ranked link completion tx: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			s.logf("[ranked] complete link db=%s steam=%s code=%s branch=rollback_error err=%v", s.dbPath, maskIdentifier(steamID), code, rollbackErr)
			return
		}
		s.logf("[ranked] complete link db=%s steam=%s code=%s branch=rollback", s.dbPath, maskIdentifier(steamID), code)
	}()
	s.logf("[ranked] complete link db=%s steam=%s code=%s branch=tx_started", s.dbPath, maskIdentifier(steamID), code)

	expireResult, err := tx.ExecContext(ctx, `
		UPDATE ranked_link_sessions
		SET status = ?, used = 1
		WHERE status = ? AND used = 0 AND expires_at <= ?`, linkSessionExpired, linkSessionPending, now.Format(time.RFC3339))
	if err != nil {
		s.logf("[ranked] complete link db=%s steam=%s code=%s branch=expire_sessions_error err=%v", s.dbPath, maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, fmt.Errorf("expire ranked link sessions: %w", err)
	}
	if expiredRows, rowsErr := expireResult.RowsAffected(); rowsErr == nil {
		s.logf("[ranked] complete link db=%s steam=%s code=%s branch=expire_sessions rows=%d", s.dbPath, maskIdentifier(steamID), code, expiredRows)
	}

	session, err := loadLinkSessionByCode(ctx, tx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logf("[ranked] complete link db=%s steam=%s code=%s branch=session_not_found", s.dbPath, maskIdentifier(steamID), code)
			return models.RankedLinkCompleteResult{}, ErrInvalidLinkCode
		}
		s.logf("[ranked] complete link db=%s steam=%s code=%s branch=session_load_error err=%v", s.dbPath, maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, err
	}
	s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=session_loaded status=%s used=%t", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, session.Status, session.Used)

	if session.Used || session.Status == linkSessionCompleted {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=already_used", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code)
		return models.RankedLinkCompleteResult{}, ErrLinkCodeAlreadyUsed
	}
	if session.Status == linkSessionExpired || now.After(session.ExpiresAt) {
		if session.Status == linkSessionPending && now.After(session.ExpiresAt) {
			if _, err := tx.ExecContext(ctx, `UPDATE ranked_link_sessions SET status = ?, used = 1 WHERE code = ?`, linkSessionExpired, code); err != nil {
				s.logf("[ranked] complete link db=%s steam=%s code=%s branch=mark_expired_error err=%v", s.dbPath, maskIdentifier(steamID), code, err)
				return models.RankedLinkCompleteResult{}, fmt.Errorf("mark ranked link session expired: %w", err)
			}
		}
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=expired", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code)
		return models.RankedLinkCompleteResult{}, ErrLinkCodeExpired
	}
	if session.Status != linkSessionPending {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=invalid_status status=%s", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, session.Status)
		return models.RankedLinkCompleteResult{}, ErrLinkCodeAlreadyUsed
	}

	discordDisplay, existingGameName, existingGamePlayerNumber, err := currentLinkPresentation(ctx, tx, session.DiscordID, steamID)
	if err != nil {
		return models.RankedLinkCompleteResult{}, err
	}

	if gameDisplayName == "" {
		gameDisplayName = existingGameName
	}
	if gamePlayerNumber == "" {
		gamePlayerNumber = existingGamePlayerNumber
	}
	s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=resolved_game_fields raw_game_player_number=%q normalized_game_player_number=%q resolved_game_player_number=%q", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, rawGamePlayerNumber, sanitizeGamePlayerNumber(gamePlayerNumber), gamePlayerNumber)

	rankedProfileInitialized := false
	if !rankedProfileExists {
		initialized, initErr := initializeRankedProfileTx(ctx, tx, steamID, firstNonEmptyString(gameDisplayName, existingGameName))
		if initErr != nil {
			s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=initialize_ranked_profile_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, initErr)
			return models.RankedLinkCompleteResult{}, initErr
		}
		rankedProfileInitialized = initialized
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=initialize_ranked_profile existed=%t initialized=%t mmr=%d tier=%s", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, rankedProfileExists, rankedProfileInitialized, ranktier.InitialMMR, ranktier.Resolve(ranktier.InitialMMR).TierKey)
	}

	deleteResult, err := tx.ExecContext(ctx, `DELETE FROM discord_links WHERE discord_id = ? OR steam_id = ?`, session.DiscordID, steamID)
	if err != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=delete_previous_links_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, fmt.Errorf("delete previous discord links: %w", err)
	}
	if deletedRows, rowsErr := deleteResult.RowsAffected(); rowsErr == nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=delete_previous_links rows=%d", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, deletedRows)
	}

	insertResult, err := tx.ExecContext(ctx, `
		INSERT INTO discord_links (discord_id, steam_id, discord_display, last_known_game_name, last_known_game_player_number, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, session.DiscordID, steamID, nullableString(discordDisplay), nullableString(gameDisplayName), nullableString(gamePlayerNumber))
	if err != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=insert_discord_link_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, fmt.Errorf("insert completed discord link: %w", err)
	}
	insertedRows, rowsErr := insertResult.RowsAffected()
	if rowsErr != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=insert_discord_link_rows_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, rowsErr)
		return models.RankedLinkCompleteResult{}, fmt.Errorf("discord link insert rows affected: %w", rowsErr)
	}
	if insertedRows <= 0 {
		err = fmt.Errorf("discord link insert produced no rows")
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=insert_discord_link_zero_rows err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, err
	}
	persistedLinkCount, persistErr := countDiscordLinksTx(ctx, tx, session.DiscordID, steamID)
	if persistErr != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=verify_discord_link_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, persistErr)
		return models.RankedLinkCompleteResult{}, persistErr
	}
	if persistedLinkCount <= 0 {
		err = fmt.Errorf("discord link was not persisted")
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=verify_discord_link_missing err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, err
	}
	persistedIdentity, persistIdentityErr := loadPersistedDiscordLinkTx(ctx, tx, session.DiscordID, steamID)
	if persistIdentityErr != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=verify_discord_link_fields_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, persistIdentityErr)
		return models.RankedLinkCompleteResult{}, persistIdentityErr
	}
	s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=insert_discord_link result=ok rows=%d persisted_count=%d", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, insertedRows, persistedLinkCount)
	s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=insert_discord_link_fields linked_identity_loaded=true discord_display=%q last_known_game_name=%q last_known_game_player_number=%q", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, persistedIdentity.DiscordDisplay, persistedIdentity.LastKnownGameName, persistedIdentity.LastKnownGamePlayerNumber)

	completeResult, err := tx.ExecContext(ctx, `
		UPDATE ranked_link_sessions
		SET status = ?, used = 1, steam_id = ?, completed_at = ?
		WHERE code = ?`, linkSessionCompleted, steamID, now.Format(time.RFC3339), code)
	if err != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=complete_session_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, fmt.Errorf("complete ranked link session: %w", err)
	}
	completedRows, rowsErr := completeResult.RowsAffected()
	if rowsErr != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=complete_session_rows_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, rowsErr)
		return models.RankedLinkCompleteResult{}, fmt.Errorf("completed ranked link session rows affected: %w", rowsErr)
	}
	if completedRows != 1 {
		err = fmt.Errorf("expected 1 ranked link session to be completed, got %d", completedRows)
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=complete_session_unexpected_rows err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, err
	}
	statusValue, usedValue, persistedSteamID, stateErr := loadPersistedLinkSessionStateTx(ctx, tx, code)
	if stateErr != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=verify_session_state_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, stateErr)
		return models.RankedLinkCompleteResult{}, stateErr
	}
	if statusValue != linkSessionCompleted || !usedValue || persistedSteamID != steamID {
		err = fmt.Errorf("ranked link session completion state was not persisted")
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=verify_session_state_mismatch status=%s used=%t persisted_steam=%s err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, statusValue, usedValue, maskIdentifier(persistedSteamID), err)
		return models.RankedLinkCompleteResult{}, err
	}
	s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=complete_session result=ok rows=%d", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, completedRows)

	siblingResult, err := tx.ExecContext(ctx, `
		UPDATE ranked_link_sessions
		SET status = ?, used = 1
		WHERE discord_id = ? AND status = ? AND code <> ?`, linkSessionCancelled, session.DiscordID, linkSessionPending, code)
	if err != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=cancel_siblings_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, fmt.Errorf("cancel sibling ranked link sessions: %w", err)
	}
	if siblingRows, rowsErr := siblingResult.RowsAffected(); rowsErr == nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=cancel_siblings rows=%d", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, siblingRows)
	}

	if err := tx.Commit(); err != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=commit_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, err)
		return models.RankedLinkCompleteResult{}, fmt.Errorf("commit ranked link completion: %w", err)
	}
	committed = true
	s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=commit_success", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code)

	finalPlayer, finalPlayerErr := s.Rank(ctx, steamID, "", "")
	if finalPlayerErr != nil {
		s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=final_rank_lookup_error err=%v", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, finalPlayerErr)
		return models.RankedLinkCompleteResult{}, finalPlayerErr
	}
	identity := &models.RankedLinkedIdentity{
		DiscordID:                 session.DiscordID,
		SteamID:                   steamID,
		DiscordDisplay:            finalPlayer.LinkedDiscordDisplay,
		LastKnownGameName:         finalPlayer.LastKnownGameName,
		LastKnownGamePlayerNumber: finalPlayer.LastKnownGamePlayerNumber,
	}
	result := models.RankedLinkCompleteResult{
		OK:        true,
		DiscordID: session.DiscordID,
		SteamID:   steamID,
		Identity:  identity,
		Player:    &finalPlayer,
	}
	s.logf("[ranked] complete link db=%s discord=%s steam=%s code=%s branch=final_response ranked_profile_existed=%t ranked_profile_initialized=%t resulting_mmr=%d resulting_tier=%s response_name=%q response_player_number=%q", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, rankedProfileExists, rankedProfileInitialized, finalPlayer.MMR, finalPlayer.Tier.TierKey, identity.LastKnownGameName, identity.LastKnownGamePlayerNumber)

	s.logf("[ranked] completed link db=%s discord=%s steam=%s code=%s stored_game_name=%t stored_game_player_number=%t final_api_game_player_number=%q", s.dbPath, maskIdentifier(session.DiscordID), maskIdentifier(steamID), code, strings.TrimSpace(gameDisplayName) != "", strings.TrimSpace(gamePlayerNumber) != "", gamePlayerNumber)
	return result, nil
}

func (s *Service) GetSteamByDiscord(ctx context.Context, discordID string) (string, error) {
	discordID = normalizeDigits(discordID)
	if err := validateDiscordID(discordID); err != nil {
		return "", err
	}

	steamID, linked, err := s.lookupPermanentLinkByDiscord(ctx, discordID)
	if err != nil {
		return "", err
	}
	if !linked {
		return "", sql.ErrNoRows
	}

	return steamID, nil
}

func (s *Service) lookupPermanentLinkByDiscord(ctx context.Context, discordID string) (string, bool, error) {
	discordID = normalizeDigits(discordID)
	if err := validateDiscordID(discordID); err != nil {
		return "", false, err
	}

	var steamID string
	err := s.db.QueryRowContext(ctx, `SELECT steam_id FROM discord_links WHERE discord_id = ?`, discordID).Scan(&steamID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}

		return "", false, fmt.Errorf("lookup steam by discord id: %w", err)
	}

	steamID = normalizeDigits(steamID)
	if steamID == "" {
		return "", false, nil
	}

	return steamID, true, nil
}

func (s *Service) loadSnapshot(ctx context.Context) ([]models.RankedLeaderboardEntry, error) {
	entries, ok := s.loadFromSourceFiles()
	if !ok {
		if s.dataPath == "" {
			entries = s.cachedSnapshot()
		} else {
			content, err := os.ReadFile(s.dataPath)
			if err != nil {
				if s.logger != nil {
					s.logger.Printf("[ranked] unable to read ranked snapshot %s: %v", s.dataPath, err)
				}
				entries = s.cachedSnapshot()
			} else {
				parsedEntries, parseErr := parseSnapshot(content)
				if parseErr != nil {
					if s.logger != nil {
						s.logger.Printf("[ranked] unable to parse ranked snapshot %s: %v", s.dataPath, parseErr)
					}
					entries = s.cachedSnapshot()
				} else {
					entries = parsedEntries
				}
			}
		}
	}

	mergedEntries, err := s.mergePersistedProfiles(ctx, entries)
	if err != nil {
		return nil, err
	}

	normalized := normalizeEntries(mergedEntries)
	if len(normalized) == 0 {
		return s.cachedSnapshot(), nil
	}

	s.mu.Lock()
	s.cached = append([]models.RankedLeaderboardEntry(nil), normalized...)
	s.mu.Unlock()

	return normalized, nil
}

func (s *Service) loadFromSourceFiles() ([]models.RankedLeaderboardEntry, bool) {
	if s.mmrPath == "" {
		return nil, false
	}

	mmrContent, err := os.ReadFile(s.mmrPath)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("[ranked] unable to read ranked mmr file %s: %v", s.mmrPath, err)
		}
		return nil, false
	}

	mmrEntries, err := parseMMRFile(mmrContent)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("[ranked] unable to parse ranked mmr file %s: %v", s.mmrPath, err)
		}
		return nil, false
	}

	starsEntries := map[string]starsFileEntry{}
	if s.starsPath != "" {
		starsContent, err := os.ReadFile(s.starsPath)
		if err != nil {
			if s.logger != nil {
				s.logger.Printf("[ranked] unable to read ranked stars file %s: %v", s.starsPath, err)
			}
		} else {
			parsedStars, parseErr := parseStarsFile(starsContent)
			if parseErr != nil {
				if s.logger != nil {
					s.logger.Printf("[ranked] unable to parse ranked stars file %s: %v", s.starsPath, parseErr)
				}
			} else {
				starsEntries = parsedStars
			}
		}
	}

	merged := mergeEntries(mmrEntries, starsEntries)
	if len(merged) == 0 {
		return nil, false
	}

	normalized := normalizeEntries(merged)
	if len(normalized) == 0 {
		return nil, false
	}

	s.mu.Lock()
	s.cached = append([]models.RankedLeaderboardEntry(nil), normalized...)
	s.mu.Unlock()

	return normalized, true
}

func parseSnapshot(content []byte) ([]models.RankedLeaderboardEntry, error) {
	var direct []models.RankedLeaderboardEntry
	if err := json.Unmarshal(content, &direct); err == nil && len(direct) > 0 {
		return direct, nil
	}

	var wrapper snapshotFile
	if err := json.Unmarshal(content, &wrapper); err != nil {
		return nil, fmt.Errorf("parse ranked snapshot: %w", err)
	}

	switch {
	case len(wrapper.Entries) > 0:
		return wrapper.Entries, nil
	case len(wrapper.Players) > 0:
		return wrapper.Players, nil
	case len(wrapper.Leaderboard) > 0:
		return wrapper.Leaderboard, nil
	default:
		return nil, nil
	}
}

func parseMMRFile(content []byte) (map[string]mmrFileEntry, error) {
	direct := map[string]mmrFileEntry{}
	if err := json.Unmarshal(content, &direct); err == nil && len(direct) > 0 {
		return direct, nil
	}

	var wrapper mmrFileWrapper
	if err := json.Unmarshal(content, &wrapper); err != nil {
		return nil, fmt.Errorf("parse ranked mmr file: %w", err)
	}

	if len(wrapper.Players) > 0 {
		return wrapper.Players, nil
	}
	if len(wrapper.Data) > 0 {
		return wrapper.Data, nil
	}

	return map[string]mmrFileEntry{}, nil
}

func parseStarsFile(content []byte) (map[string]starsFileEntry, error) {
	direct := map[string]starsFileEntry{}
	if err := json.Unmarshal(content, &direct); err == nil && len(direct) > 0 {
		return direct, nil
	}

	var wrapper starsFileWrapper
	if err := json.Unmarshal(content, &wrapper); err != nil {
		return nil, fmt.Errorf("parse ranked stars file: %w", err)
	}

	if len(wrapper.Players) > 0 {
		return wrapper.Players, nil
	}
	if len(wrapper.Data) > 0 {
		return wrapper.Data, nil
	}

	return map[string]starsFileEntry{}, nil
}

func mergeEntries(mmrEntries map[string]mmrFileEntry, starsEntries map[string]starsFileEntry) []models.RankedLeaderboardEntry {
	merged := make([]models.RankedLeaderboardEntry, 0, len(mmrEntries))
	for steamID, mmrEntry := range mmrEntries {
		steamID = strings.TrimSpace(steamID)
		if steamID == "" {
			continue
		}

		starsEntry, ok := starsEntries[steamID]
		if !ok {
			starsEntry = starsFileEntry{}
		}

		displayName := safeDisplayName(strings.TrimSpace(mmrEntry.DisplayName), steamID)
		merged = append(merged, models.RankedLeaderboardEntry{
			SteamID:     steamID,
			DisplayName: displayName,
			MMR:         mmrEntry.MMR,
			Wins:        mmrEntry.Wins,
			Losses:      mmrEntry.Losses,
			StarPoints:  maxInt(starsEntry.StarPoints, 0),
			WinStreak:   maxInt(starsEntry.WinStreak, 0),
		})
	}

	return merged
}

func (s *Service) mergePersistedProfiles(ctx context.Context, entries []models.RankedLeaderboardEntry) ([]models.RankedLeaderboardEntry, error) {
	persistedProfiles, err := s.loadPersistedProfiles(ctx)
	if err != nil {
		return nil, err
	}
	if len(persistedProfiles) == 0 {
		return append([]models.RankedLeaderboardEntry(nil), entries...), nil
	}

	merged := append([]models.RankedLeaderboardEntry(nil), entries...)
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.SteamID == "" {
			continue
		}
		seen[strings.TrimSpace(entry.SteamID)] = struct{}{}
	}

	for _, profile := range persistedProfiles {
		if _, ok := seen[profile.SteamID]; !ok {
			merged = append(merged, models.RankedLeaderboardEntry{
				SteamID:          profile.SteamID,
				DisplayName:      profile.DisplayName,
				MMR:              profile.MMR,
				Goals:            profile.Goals,
				Assists:          profile.Assists,
				SecondaryAssists: profile.SecondaryAssists,
				Wins:             profile.Wins,
				Losses:           profile.Losses,
				StarPoints:       profile.StarPoints,
				WinStreak:        profile.WinStreak,
			})
			continue
		}

		for index := range merged {
			if merged[index].SteamID != profile.SteamID {
				continue
			}
			merged[index].Goals = profile.Goals
			merged[index].Assists = profile.Assists
			merged[index].SecondaryAssists = profile.SecondaryAssists
			if strings.TrimSpace(merged[index].DisplayName) == "" && profile.DisplayName != "" {
				merged[index].DisplayName = profile.DisplayName
			}
			break
		}
	}

	return merged, nil
}

func (s *Service) loadPersistedProfiles(ctx context.Context) ([]persistedRankedProfile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT steam_id, COALESCE(display_name, ''), mmr, goals, assists, secondary_assists, wins, losses, star_points, win_streak
		FROM ranked_profiles`)
	if err != nil {
		return nil, fmt.Errorf("query persisted ranked profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]persistedRankedProfile, 0, 16)
	for rows.Next() {
		var profile persistedRankedProfile
		if err := rows.Scan(&profile.SteamID, &profile.DisplayName, &profile.MMR, &profile.Goals, &profile.Assists, &profile.SecondaryAssists, &profile.Wins, &profile.Losses, &profile.StarPoints, &profile.WinStreak); err != nil {
			return nil, fmt.Errorf("scan persisted ranked profile: %w", err)
		}
		profile.SteamID = strings.TrimSpace(profile.SteamID)
		profile.DisplayName = strings.TrimSpace(profile.DisplayName)
		if profile.SteamID == "" {
			continue
		}
		profiles = append(profiles, profile)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate persisted ranked profiles: %w", err)
	}

	return profiles, nil
}

func normalizeEntries(entries []models.RankedLeaderboardEntry) []models.RankedLeaderboardEntry {
	normalized := make([]models.RankedLeaderboardEntry, 0, len(entries))
	for _, entry := range entries {
		entry.SteamID = strings.TrimSpace(entry.SteamID)
		entry.DiscordID = strings.TrimSpace(entry.DiscordID)
		entry.DisplayName = safeDisplayName(strings.TrimSpace(entry.DisplayName), entry.SteamID)
		if entry.SteamID == "" {
			continue
		}
		if entry.StarPoints < 0 {
			entry.StarPoints = 0
		}
		if entry.WinStreak < 0 {
			entry.WinStreak = 0
		}
		entry.Tier = ranktier.Resolve(entry.MMR)
		normalized = append(normalized, entry)
	}

	sort.SliceStable(normalized, func(left, right int) bool {
		if normalized[left].MMR != normalized[right].MMR {
			return normalized[left].MMR > normalized[right].MMR
		}
		if normalized[left].Wins != normalized[right].Wins {
			return normalized[left].Wins > normalized[right].Wins
		}
		if normalized[left].StarPoints != normalized[right].StarPoints {
			return normalized[left].StarPoints > normalized[right].StarPoints
		}
		return strings.ToLower(normalized[left].DisplayName) < strings.ToLower(normalized[right].DisplayName)
	})

	for index := range normalized {
		normalized[index].Rank = index + 1
	}

	return normalized
}

func toRankedPlayer(entry models.RankedLeaderboardEntry) models.RankedPlayer {
	return models.RankedPlayer{
		SteamID:                   entry.SteamID,
		DiscordID:                 entry.LinkedDiscordID,
		LinkedDiscordDisplay:      entry.LinkedDiscordDisplay,
		LastKnownGameName:         entry.LastKnownGameName,
		LastKnownGamePlayerNumber: entry.LastKnownGamePlayerNumber,
		DisplayName:               entry.DisplayName,
		MMR:                       entry.MMR,
		Goals:                     entry.Goals,
		Assists:                   entry.Assists,
		SecondaryAssists:          entry.SecondaryAssists,
		Wins:                      entry.Wins,
		Losses:                    entry.Losses,
		StarPoints:                entry.StarPoints,
		WinStreak:                 entry.WinStreak,
		RankPosition:              entry.Rank,
		Tier:                      entry.Tier,
	}
}

func (s *Service) findRankedPlayerBySteamID(ctx context.Context, steamID string) (models.RankedPlayer, bool, error) {
	entries, err := s.loadSnapshot(ctx)
	if err != nil {
		return models.RankedPlayer{}, false, err
	}

	steamID = strings.TrimSpace(steamID)
	for _, entry := range entries {
		if entry.SteamID == steamID {
			return toRankedPlayer(entry), true, nil
		}
	}

	return models.RankedPlayer{}, false, nil
}

func (s *Service) cachedSnapshot() []models.RankedLeaderboardEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.cached) == 0 {
		return nil
	}

	return append([]models.RankedLeaderboardEntry(nil), s.cached...)
}

func defaultSnapshot() []models.RankedLeaderboardEntry {
	return normalizeEntries([]models.RankedLeaderboardEntry{
		{
			SteamID:     "76561199046098825",
			DisplayName: "schrader",
			MMR:         507,
			Wins:        33,
			Losses:      21,
			StarPoints:  5,
			WinStreak:   1,
		},
	})
}

func safeDisplayName(displayName string, steamID string) string {
	displayName = strings.TrimSpace(displayName)
	if displayName != "" {
		return displayName
	}

	steamID = strings.TrimSpace(steamID)
	if len(steamID) >= 6 {
		return "Player " + steamID[len(steamID)-6:]
	}
	if steamID != "" {
		return "Player " + steamID
	}

	return "Unknown Player"
}

func maxInt(value int, minimum int) int {
	if value < minimum {
		return minimum
	}

	return value
}

func (s *Service) enrichEntriesWithLinks(ctx context.Context, entries []models.RankedLeaderboardEntry) ([]models.RankedLeaderboardEntry, error) {
	if len(entries) == 0 {
		return entries, nil
	}

	links, err := s.linksBySteamID(ctx, entries)
	if err != nil {
		return nil, err
	}

	enriched := make([]models.RankedLeaderboardEntry, 0, len(entries))
	for _, entry := range entries {
		if link, ok := links[entry.SteamID]; ok {
			entry.DiscordID = link.DiscordID
			entry.LinkedDiscordID = link.DiscordID
			entry.LinkedDiscordDisplay = safeDiscordDisplay(link.DiscordDisplay, link.DiscordID)
			entry.LastKnownGameName = link.LastKnownGameName
			entry.LastKnownGamePlayerNumber = link.LastKnownGamePlayerNumber
		}
		enriched = append(enriched, entry)
	}

	return enriched, nil
}

func (s *Service) linksBySteamID(ctx context.Context, entries []models.RankedLeaderboardEntry) (map[string]discordLink, error) {
	steamIDs := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.SteamID == "" {
			continue
		}
		if _, ok := seen[entry.SteamID]; ok {
			continue
		}
		seen[entry.SteamID] = struct{}{}
		steamIDs = append(steamIDs, entry.SteamID)
	}

	if len(steamIDs) == 0 {
		return map[string]discordLink{}, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(steamIDs)), ",")
	arguments := make([]any, 0, len(steamIDs))
	for _, steamID := range steamIDs {
		arguments = append(arguments, steamID)
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT discord_id, steam_id, COALESCE(discord_display, ''), COALESCE(last_known_game_name, ''), COALESCE(last_known_game_player_number, '')
		FROM discord_links
		WHERE steam_id IN (%s)`, placeholders), arguments...)
	if err != nil {
		return nil, fmt.Errorf("query discord links by steam id: %w", err)
	}
	defer rows.Close()

	links := make(map[string]discordLink, len(steamIDs))
	for rows.Next() {
		var link discordLink
		if err := rows.Scan(&link.DiscordID, &link.SteamID, &link.DiscordDisplay, &link.LastKnownGameName, &link.LastKnownGamePlayerNumber); err != nil {
			return nil, fmt.Errorf("scan discord link: %w", err)
		}
		link.LastKnownGameName = sanitizeGameDisplayName(link.LastKnownGameName)
		link.LastKnownGamePlayerNumber = sanitizeGamePlayerNumber(link.LastKnownGamePlayerNumber)
		links[link.SteamID] = link
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate discord links: %w", err)
	}

	return links, nil
}

func (s *Service) expirePendingSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE ranked_link_sessions
		SET status = ?, used = 1
		WHERE status = ? AND used = 0 AND expires_at <= ?`, linkSessionExpired, linkSessionPending, now.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("expire pending ranked link sessions: %w", err)
	}

	return nil
}

func (s *Service) findPendingSessionByDiscord(ctx context.Context, discordID string, now time.Time) (linkSession, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT code, discord_id, COALESCE(guild_id, ''), COALESCE(channel_id, ''), status, created_at, expires_at, completed_at, used, COALESCE(steam_id, '')
		FROM ranked_link_sessions
		WHERE discord_id = ? AND status = ? AND used = 0 AND expires_at > ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, discordID, linkSessionPending, now.Format(time.RFC3339))

	session, err := scanLinkSession(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return linkSession{}, false, nil
		}

		return linkSession{}, false, err
	}

	return session, true, nil
}

func (s *Service) updatePendingSessionContext(ctx context.Context, code string, guildID string, channelID string) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE ranked_link_sessions
		SET guild_id = ?, channel_id = ?
		WHERE code = ?`, nullableString(guildID), nullableString(channelID), code); err != nil {
		return fmt.Errorf("update ranked link session context: %w", err)
	}

	return nil
}

func (s *Service) countLinkSessionsByCode(ctx context.Context, code string) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ranked_link_sessions WHERE code = ?`, code).Scan(&count); err != nil {
		return 0, fmt.Errorf("count ranked link sessions by code: %w", err)
	}

	return count, nil
}

func (s *Service) cancelPendingSessionsForDiscord(ctx context.Context, discordID string) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE ranked_link_sessions
		SET status = ?, used = 1
		WHERE discord_id = ? AND status = ? AND used = 0`, linkSessionCancelled, discordID, linkSessionPending); err != nil {
		return fmt.Errorf("cancel pending ranked link sessions for discord: %w", err)
	}

	return nil
}

func currentLinkPresentation(ctx context.Context, tx *sql.Tx, discordID string, steamID string) (string, string, string, error) {
	var display string
	var gameName string
	var gamePlayerNumber string
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(discord_display, ''), COALESCE(last_known_game_name, ''), COALESCE(last_known_game_player_number, '')
		FROM discord_links
		WHERE discord_id = ? OR steam_id = ?
		ORDER BY CASE WHEN discord_id = ? THEN 0 ELSE 1 END
		LIMIT 1`, discordID, steamID, discordID).Scan(&display, &gameName, &gamePlayerNumber)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", "", nil
		}

		return "", "", "", fmt.Errorf("load current link presentation: %w", err)
	}

	return strings.TrimSpace(display), sanitizeGameDisplayName(gameName), sanitizeGamePlayerNumber(gamePlayerNumber), nil
}

func countDiscordLinksTx(ctx context.Context, tx *sql.Tx, discordID string, steamID string) (int, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM discord_links WHERE discord_id = ? AND steam_id = ?`, discordID, steamID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count persisted discord links: %w", err)
	}

	return count, nil
}

func loadPersistedDiscordLinkTx(ctx context.Context, tx *sql.Tx, discordID string, steamID string) (discordLink, error) {
	var link discordLink
	if err := tx.QueryRowContext(ctx, `
		SELECT discord_id, steam_id, COALESCE(discord_display, ''), COALESCE(last_known_game_name, ''), COALESCE(last_known_game_player_number, '')
		FROM discord_links
		WHERE discord_id = ? AND steam_id = ?
		LIMIT 1`, discordID, steamID).Scan(&link.DiscordID, &link.SteamID, &link.DiscordDisplay, &link.LastKnownGameName, &link.LastKnownGamePlayerNumber); err != nil {
		return discordLink{}, fmt.Errorf("load persisted discord link fields: %w", err)
	}

	link.DiscordDisplay = strings.TrimSpace(link.DiscordDisplay)
	link.LastKnownGameName = sanitizeGameDisplayName(link.LastKnownGameName)
	link.LastKnownGamePlayerNumber = sanitizeGamePlayerNumber(link.LastKnownGamePlayerNumber)
	return link, nil
}

func initializeRankedProfileTx(ctx context.Context, tx *sql.Tx, steamID string, displayName string) (bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO ranked_profiles (steam_id, display_name, mmr, goals, assists, secondary_assists, wins, losses, star_points, win_streak, created_at, updated_at)
		VALUES (?, ?, ?, 0, 0, 0, 0, 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		steamID,
		nullableString(displayName),
		ranktier.InitialMMR,
	)
	if err != nil {
		return false, fmt.Errorf("initialize ranked profile: %w", err)
	}

	rowsAffected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return false, fmt.Errorf("initialize ranked profile rows affected: %w", rowsErr)
	}

	return rowsAffected > 0, nil
}

func loadPersistedLinkSessionStateTx(ctx context.Context, tx *sql.Tx, code string) (string, bool, string, error) {
	var status string
	var used int
	var steamID string
	if err := tx.QueryRowContext(ctx, `
		SELECT status, used, COALESCE(steam_id, '')
		FROM ranked_link_sessions
		WHERE code = ?`, code).Scan(&status, &used, &steamID); err != nil {
		return "", false, "", fmt.Errorf("load persisted ranked link session state: %w", err)
	}

	return strings.TrimSpace(status), used == 1, normalizeDigits(steamID), nil
}

type linkSessionScanner interface {
	Scan(dest ...any) error
}

func loadLinkSessionByCode(ctx context.Context, tx *sql.Tx, code string) (linkSession, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT code, discord_id, COALESCE(guild_id, ''), COALESCE(channel_id, ''), status, created_at, expires_at, completed_at, used, COALESCE(steam_id, '')
		FROM ranked_link_sessions
		WHERE code = ?`, code)

	return scanLinkSession(row)
}

func scanLinkSession(row linkSessionScanner) (linkSession, error) {
	var session linkSession
	var createdAt string
	var expiresAt string
	var used int

	if err := row.Scan(
		&session.Code,
		&session.DiscordID,
		&session.GuildID,
		&session.ChannelID,
		&session.Status,
		&createdAt,
		&expiresAt,
		&session.CompletedAt,
		&used,
		&session.SteamID,
	); err != nil {
		return linkSession{}, err
	}

	parsedCreatedAt, err := parseStoredTime(createdAt)
	if err != nil {
		return linkSession{}, fmt.Errorf("parse ranked link created_at: %w", err)
	}
	parsedExpiresAt, err := parseStoredTime(expiresAt)
	if err != nil {
		return linkSession{}, fmt.Errorf("parse ranked link expires_at: %w", err)
	}

	session.CreatedAt = parsedCreatedAt
	session.ExpiresAt = parsedExpiresAt
	session.Used = used == 1

	return session, nil
}

func parseStoredTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("timestamp is empty")
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported timestamp format")
}

func secondsUntil(expiresAt time.Time, now time.Time) int {
	remaining := int(expiresAt.Sub(now).Seconds())
	if remaining < 0 {
		return 0
	}
	return remaining
}

func normalizeLinkCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func generateLinkCode() (string, error) {
	var buffer [4]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return "", fmt.Errorf("generate link code entropy: %w", err)
	}

	value := binary.BigEndian.Uint32(buffer[:])%900000 + 100000
	return fmt.Sprintf("SR-%06d", value), nil
}

func isUniqueCodeError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "ranked_link_sessions.code")
}

func isPendingDiscordConflict(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "ranked_link_sessions.discord_id") || strings.Contains(message, "ux_ranked_link_sessions_pending_discord")
}

func maskIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4 {
		return value
	}
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}

func maskCode(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 3 {
		return value
	}
	return value[:3] + strings.Repeat("*", len(value)-3)
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}

func validateDiscordID(value string) error {
	if value == "" {
		return fmt.Errorf("discord id is required")
	}
	if len(value) < 5 || len(value) > 32 {
		return fmt.Errorf("discord id must be between 5 and 32 digits")
	}
	return nil
}

func validateSteamID(value string) error {
	if value == "" {
		return fmt.Errorf("steam id is required")
	}
	if len(value) < 5 || len(value) > 32 {
		return fmt.Errorf("steam id must be between 5 and 32 digits")
	}
	return nil
}

func normalizeDigits(value string) string {
	value = strings.TrimSpace(value)
	buffer := strings.Builder{}
	for _, character := range value {
		if character >= '0' && character <= '9' {
			buffer.WriteRune(character)
		}
	}
	return buffer.String()
}

func safeDiscordDisplay(display string, discordID string) string {
	display = strings.TrimSpace(display)
	if display != "" {
		return display
	}
	discordID = strings.TrimSpace(discordID)
	if len(discordID) >= 4 {
		return "Discord " + discordID[len(discordID)-4:]
	}
	if discordID != "" {
		return "Discord " + discordID
	}
	return "Discord User"
}

func sanitizeGameDisplayName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if len(value) > 64 {
		value = value[:64]
	}

	return value
}

func sanitizeGamePlayerNumber(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if len(value) > 32 {
		value = value[:32]
	}

	return value
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrPlayerNotFound) || errors.Is(err, sql.ErrNoRows)
}
