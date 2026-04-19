package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"speedhosting/backend/internal/config"
	"speedhosting/backend/internal/models"
	analyticsservice "speedhosting/backend/internal/services/analytics"
	planservice "speedhosting/backend/internal/services/plan"
)

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

var (
	ErrInvalidServerInput   = errors.New("invalid server input")
	ErrPlanLimitReached     = errors.New("plan limit reached")
	ErrDuplicateServerFile  = errors.New("server config file already exists")
	ErrRuntimeTemplate      = errors.New("runtime template unavailable")
	ErrRuntimeActionFailed  = errors.New("server runtime action failed")
	ErrRuntimeCleanupFailed = errors.New("server runtime cleanup failed")
	ErrUnsupportedAction    = errors.New("unsupported action")
)

const (
	fixedRegionLabel = "Nuremberg"
)

var adminControlPlan = models.Plan{
	Code:                    "admin",
	Name:                    "Admin",
	MaxServers:              999,
	MaxTickRate:             1000,
	MaxAdmins:               128,
	MaxAdminSteamIDs:        128,
	AllowCustomMods:         true,
	AllowAdvancedConfig:     true,
	MaxUserConfigurableMods: 64,
	AllowSpeedRankeds:       true,
	PremiumFeatureAccess:    true,
}

type Service struct {
	db        *sql.DB
	plans     *planservice.Service
	analytics *analyticsservice.Service
	logger    *log.Logger
	cfg       config.Config
}

type CreateInput struct {
	Name            string                        `json:"name"`
	Region          string                        `json:"region"`
	DesiredTickRate int                           `json:"desiredTickRate"`
	MaxPlayers      int                           `json:"maxPlayers"`
	Password        string                        `json:"password"`
	ServerMode      string                        `json:"serverMode,omitempty"`
	AdminSteamIDs   []string                      `json:"adminSteamIds"`
	Mods            []models.ServerConfigMod      `json:"mods"`
	Attribution     models.AcquisitionAttribution `json:"acquisition"`
}

type UpdateConfigInput struct {
	Config models.ServerConfig `json:"config"`
}

func NewService(db *sql.DB, plans *planservice.Service, analytics *analyticsservice.Service, logger *log.Logger, cfg config.Config) *Service {
	return &Service{db: db, plans: plans, analytics: analytics, logger: logger, cfg: cfg}
}

func (s *Service) ListByOwner(ctx context.Context, ownerID int64) ([]models.Server, error) {
	const query = `
		SELECT s.id, s.owner_id, s.name, s.slug, s.region, s.config_file_path, s.service_name, s.status, s.desired_tick_rate,
		       s.max_players, COALESCE(r.player_count, 0), COALESCE(r.process_state, s.status), COALESCE(r.last_action_error, ''), s.config_json
		FROM servers s
		LEFT JOIN server_runtime r ON r.server_id = s.id
		WHERE s.owner_id = ?
		ORDER BY s.created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer rows.Close()

	servers := make([]models.Server, 0)
	for rows.Next() {
		server, _, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate servers: %w", err)
	}

	return servers, nil
}

func (s *Service) GetByID(ctx context.Context, ownerID int64, serverID int64) (models.Server, error) {
	server, _, err := s.getServerRecord(ctx, ownerID, serverID)
	if err != nil {
		return models.Server{}, err
	}

	return server, nil
}

func (s *Service) ListAll(ctx context.Context) ([]models.Server, error) {
	const query = `
		SELECT s.id, s.owner_id, s.name, s.slug, s.region, s.config_file_path, s.service_name, s.status, s.desired_tick_rate,
		       s.max_players, COALESCE(r.player_count, 0), COALESCE(r.process_state, s.status), COALESCE(r.last_action_error, ''), s.config_json
		FROM servers s
		LEFT JOIN server_runtime r ON r.server_id = s.id
		ORDER BY s.created_at DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list all servers: %w", err)
	}
	defer rows.Close()

	servers := make([]models.Server, 0)
	for rows.Next() {
		server, _, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all servers: %w", err)
	}

	return servers, nil
}

func (s *Service) GetAnyByID(ctx context.Context, serverID int64) (models.Server, error) {
	server, _, err := s.getServerRecordAny(ctx, serverID)
	if err != nil {
		return models.Server{}, err
	}

	return server, nil
}

func (s *Service) getServerRecord(ctx context.Context, ownerID int64, serverID int64) (models.Server, string, error) {
	const query = `
		SELECT s.id, s.owner_id, s.name, s.slug, s.region, s.config_file_path, s.service_name, s.status, s.desired_tick_rate,
		       s.max_players, COALESCE(r.player_count, 0), COALESCE(r.process_state, s.status), COALESCE(r.last_action_error, ''), s.config_json
		FROM servers s
		LEFT JOIN server_runtime r ON r.server_id = s.id
		WHERE s.id = ? AND s.owner_id = ?`

	row := s.db.QueryRowContext(ctx, query, serverID, ownerID)
	server, rawConfigJSON, err := scanServer(row)
	if err != nil {
		return models.Server{}, "", err
	}

	return server, rawConfigJSON, nil
}

func (s *Service) getServerRecordAny(ctx context.Context, serverID int64) (models.Server, string, error) {
	const query = `
		SELECT s.id, s.owner_id, s.name, s.slug, s.region, s.config_file_path, s.service_name, s.status, s.desired_tick_rate,
		       s.max_players, COALESCE(r.player_count, 0), COALESCE(r.process_state, s.status), COALESCE(r.last_action_error, ''), s.config_json
		FROM servers s
		LEFT JOIN server_runtime r ON r.server_id = s.id
		WHERE s.id = ?`

	row := s.db.QueryRowContext(ctx, query, serverID)
	server, rawConfigJSON, err := scanServer(row)
	if err != nil {
		return models.Server{}, "", err
	}

	return server, rawConfigJSON, nil
}

func (s *Service) CreateServer(ctx context.Context, user models.AuthenticatedUser, input CreateInput) (models.Server, error) {
	if strings.TrimSpace(input.Name) == "" {
		return models.Server{}, fmt.Errorf("%w: server name is required", ErrInvalidServerInput)
	}

	input.Region = fixedRegionLabel

	if input.MaxPlayers <= 0 {
		input.MaxPlayers = 10
	}

	plan, err := s.plans.GetForUser(ctx, user.ID)
	if err != nil {
		return models.Server{}, err
	}

	var serverCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM servers WHERE owner_id = ?`, user.ID).Scan(&serverCount); err != nil {
		return models.Server{}, fmt.Errorf("count servers: %w", err)
	}
	wasFirstServer := serverCount == 0

	if serverCount >= plan.MaxServers {
		s.logf("[plan-enforcement] plan_code=%s requested_tick_rate=%d requested_admin_steam_ids=%v requested_mod_ids=%v advanced_settings_allowed=%t result=rejected reason=%q", strings.ToLower(strings.TrimSpace(plan.Code)), input.DesiredTickRate, input.AdminSteamIDs, []string{}, plan.AllowAdvancedConfig, fmt.Sprintf("max servers is %d", plan.MaxServers))
		return models.Server{}, fmt.Errorf("%w: %s allows %d server", ErrPlanLimitReached, plan.Name, plan.MaxServers)
	}

	if input.DesiredTickRate <= 0 {
		input.DesiredTickRate = plan.MaxTickRate
	}

	if input.DesiredTickRate > plan.MaxTickRate {
		s.logf("[plan-enforcement] plan_code=%s requested_tick_rate=%d requested_admin_steam_ids=%v requested_mod_ids=%v advanced_settings_allowed=%t result=rejected reason=%q", strings.ToLower(strings.TrimSpace(plan.Code)), input.DesiredTickRate, input.AdminSteamIDs, []string{}, plan.AllowAdvancedConfig, fmt.Sprintf("max tick rate is %d", plan.MaxTickRate))
		return models.Server{}, fmt.Errorf("%w: max tick rate is %d", ErrPlanLimitReached, plan.MaxTickRate)
	}

	runtimeSpec, err := s.prepareRuntimeServer(ctx, input, plan)
	if err != nil {
		return models.Server{}, err
	}

	slug, err := s.ensureUniqueSlug(ctx, uniqueSlugCandidate(input.Name))
	if err != nil {
		return models.Server{}, err
	}

	if err := s.writeRuntimeConfigFile(runtimeSpec.ConfigFilePath, runtimeSpec.ConfigJSON); err != nil {
		return models.Server{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		_ = s.deleteRuntimeConfigFile(runtimeSpec.ConfigFilePath)
		return models.Server{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO servers (owner_id, name, slug, region, config_file_path, service_name, acquisition_source, acquisition_timestamp, acquisition_session_id, status, desired_tick_rate, max_players, auto_shutdown_enabled, config_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'stopped', ?, ?, ?, ?)`,
		user.ID,
		strings.TrimSpace(input.Name),
		slug,
		input.Region,
		runtimeSpec.ConfigFilePath,
		runtimeSpec.ServiceName,
		nullableAttributionValue(input.Attribution.Source),
		nullableAttributionTimestamp(input.Attribution.Timestamp),
		nullableAttributionValue(input.Attribution.SessionID),
		input.DesiredTickRate,
		input.MaxPlayers,
		0,
		runtimeSpec.ConfigJSON,
	)
	if err != nil {
		_ = s.deleteRuntimeConfigFile(runtimeSpec.ConfigFilePath)
		return models.Server{}, fmt.Errorf("insert server: %w", err)
	}

	serverID, err := result.LastInsertId()
	if err != nil {
		_ = s.deleteRuntimeConfigFile(runtimeSpec.ConfigFilePath)
		return models.Server{}, fmt.Errorf("read server id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO server_runtime (server_id, process_state, player_count, max_players)
		VALUES (?, 'stopped', 0, ?)`, serverID, input.MaxPlayers); err != nil {
		_ = s.deleteRuntimeConfigFile(runtimeSpec.ConfigFilePath)
		return models.Server{}, fmt.Errorf("insert server runtime: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO server_admins (server_id, user_id, role)
		VALUES (?, ?, 'owner')`, serverID, user.ID); err != nil {
		_ = s.deleteRuntimeConfigFile(runtimeSpec.ConfigFilePath)
		return models.Server{}, fmt.Errorf("insert server admin: %w", err)
	}

	if err := tx.Commit(); err != nil {
		_ = s.deleteRuntimeConfigFile(runtimeSpec.ConfigFilePath)
		return models.Server{}, fmt.Errorf("commit server create: %w", err)
	}

	s.logAdminSteamIDMerge(serverID, input.Name, strings.ToLower(strings.TrimSpace(plan.Code)), runtimeSpec.TemplatePath, runtimeSpec.AdminMerge)
	s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=create result=ok final_service_name=%s", serverID, strings.TrimSpace(input.Name), strings.ToLower(strings.TrimSpace(plan.Code)), runtimeSpec.ServiceName, runtimeSpec.ServiceName)

	if wasFirstServer && s.analytics != nil {
		serverIDCopy := serverID
		_ = s.analytics.TrackEvent(ctx, &user.ID, analyticsservice.TrackEventInput{
			Name:            "first_server_created",
			Source:          input.Attribution.Source,
			Route:           firstNonEmptyAttribution(input.Attribution.Route, "/app/servers"),
			LandingPath:     input.Attribution.LandingPath,
			FullURL:         input.Attribution.FullURL,
			SessionID:       input.Attribution.SessionID,
			ClientTimestamp: input.Attribution.Timestamp,
			ServerID:        &serverIDCopy,
		})
	}

	return s.GetByID(ctx, user.ID, serverID)
}

func (s *Service) PerformAction(ctx context.Context, ownerID int64, serverID int64, action string) (models.Server, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "start" && action != "stop" && action != "restart" {
		return models.Server{}, fmt.Errorf("%w: %s", ErrUnsupportedAction, action)
	}

	server, err := s.GetByID(ctx, ownerID, serverID)
	if err != nil {
		return models.Server{}, err
	}

	planCode, serviceName, err := s.resolveRuntimeUnit(ctx, server)
	if err != nil {
		_ = s.markActionFailure(ctx, serverID, err.Error())
		return models.Server{}, fmt.Errorf("%w: %s", ErrRuntimeActionFailed, err.Error())
	}

	if err := s.performRuntimeAction(ctx, action, server, planCode, serviceName); err != nil {
		_ = s.markActionFailure(ctx, serverID, err.Error())
		return models.Server{}, fmt.Errorf("%w: %s", ErrRuntimeActionFailed, err.Error())
	}

	status := s.currentServerStatus(ctx, server, planCode, serviceName)
	playerCount := 0

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Server{}, fmt.Errorf("begin action tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE servers
		SET status = ?, service_name = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND owner_id = ?`, status, serviceName, serverID, ownerID)
	if err != nil {
		return models.Server{}, fmt.Errorf("update server status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.Server{}, fmt.Errorf("server action rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.Server{}, sql.ErrNoRows
	}

	lastStopAt := "NULL"
	lastStartAt := "NULL"
	if status == "running" {
		lastStartAt = "CURRENT_TIMESTAMP"
	} else {
		lastStopAt = "CURRENT_TIMESTAMP"
	}

	query := fmt.Sprintf(`
		UPDATE server_runtime
		SET process_state = ?, player_count = ?, updated_at = CURRENT_TIMESTAMP,
		    last_start_at = COALESCE(%s, last_start_at),
		    last_stop_at = COALESCE(%s, last_stop_at),
		    last_action_error = NULL,
		    last_empty_at = CASE WHEN ? = 0 THEN CURRENT_TIMESTAMP ELSE last_empty_at END
		WHERE server_id = ?`, lastStartAt, lastStopAt)

	if _, err := tx.ExecContext(ctx, query, status, playerCount, playerCount, serverID); err != nil {
		return models.Server{}, fmt.Errorf("update server runtime: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.Server{}, fmt.Errorf("commit server action: %w", err)
	}

	s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=%s result=ok final_service_name=%s persisted_status=%s", server.ID, server.Name, planCode, serviceName, action, serviceName, status)

	return s.GetByID(ctx, ownerID, serverID)
}

func (s *Service) PerformAdminAction(ctx context.Context, serverID int64, action string) (models.Server, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "start" && action != "stop" && action != "restart" {
		return models.Server{}, fmt.Errorf("%w: %s", ErrUnsupportedAction, action)
	}

	server, err := s.GetAnyByID(ctx, serverID)
	if err != nil {
		return models.Server{}, err
	}

	planCode, serviceName, err := s.resolveRuntimeUnit(ctx, server)
	if err != nil {
		_ = s.markActionFailure(ctx, serverID, err.Error())
		return models.Server{}, fmt.Errorf("%w: %s", ErrRuntimeActionFailed, err.Error())
	}

	if err := s.performRuntimeAction(ctx, action, server, planCode, serviceName); err != nil {
		_ = s.markActionFailure(ctx, serverID, err.Error())
		return models.Server{}, fmt.Errorf("%w: %s", ErrRuntimeActionFailed, err.Error())
	}

	status := s.currentServerStatus(ctx, server, planCode, serviceName)
	playerCount := 0

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Server{}, fmt.Errorf("begin admin action tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE servers
		SET status = ?, service_name = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, status, serviceName, serverID)
	if err != nil {
		return models.Server{}, fmt.Errorf("update admin server status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.Server{}, fmt.Errorf("admin server action rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.Server{}, sql.ErrNoRows
	}

	lastStopAt := "NULL"
	lastStartAt := "NULL"
	if status == "running" {
		lastStartAt = "CURRENT_TIMESTAMP"
	} else {
		lastStopAt = "CURRENT_TIMESTAMP"
	}

	query := fmt.Sprintf(`
		UPDATE server_runtime
		SET process_state = ?, player_count = ?, updated_at = CURRENT_TIMESTAMP,
		    last_start_at = COALESCE(%s, last_start_at),
		    last_stop_at = COALESCE(%s, last_stop_at),
		    last_action_error = NULL,
		    last_empty_at = CASE WHEN ? = 0 THEN CURRENT_TIMESTAMP ELSE last_empty_at END
		WHERE server_id = ?`, lastStartAt, lastStopAt)

	if _, err := tx.ExecContext(ctx, query, status, playerCount, playerCount, serverID); err != nil {
		return models.Server{}, fmt.Errorf("update admin server runtime: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.Server{}, fmt.Errorf("commit admin server action: %w", err)
	}

	s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=%s result=ok final_service_name=%s persisted_status=%s", server.ID, server.Name, planCode, serviceName, action, serviceName, status)

	return s.GetAnyByID(ctx, serverID)
}

func (s *Service) UpdateConfig(ctx context.Context, user models.AuthenticatedUser, serverID int64, input UpdateConfigInput) (models.Server, error) {
	plan, err := s.plans.GetForUser(ctx, user.ID)
	if err != nil {
		return models.Server{}, err
	}

	server, rawConfigJSON, err := s.getServerRecord(ctx, user.ID, serverID)
	if err != nil {
		return models.Server{}, err
	}

	payload, err := parseRuntimeConfig(rawConfigJSON)
	if err != nil {
		return models.Server{}, err
	}
	templateConfig, err := s.loadTemplateConfig()
	if err != nil {
		return models.Server{}, err
	}

	currentConfig := extractServerConfig(payload)
	templateAdminSteamIDs := mandatoryTemplateAdminSteamIDs(templateConfig)
	templateBaseMods := normalizeMods(extractServerConfig(templateConfig).Mods)
	nextConfig, adminMerge, err := s.applyHostedServerConfig(payload, currentConfig, input.Config, plan, templateAdminSteamIDs, templateBaseMods)
	if err != nil {
		return models.Server{}, err
	}

	renderedConfig, err := renderRuntimeConfig(payload)
	if err != nil {
		return models.Server{}, err
	}

	if err := s.rewriteRuntimeConfigFile(server.ConfigFilePath, renderedConfig); err != nil {
		return models.Server{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE servers
		SET desired_tick_rate = ?, max_players = ?, config_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND owner_id = ?`, nextConfig.ServerTickRate, nextConfig.MaxPlayers, renderedConfig, serverID, user.ID)
	if err != nil {
		_ = s.rewriteRuntimeConfigFile(server.ConfigFilePath, rawConfigJSON)
		return models.Server{}, fmt.Errorf("update server config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.Server{}, fmt.Errorf("config update rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return models.Server{}, sql.ErrNoRows
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE server_runtime
		SET max_players = ?, updated_at = CURRENT_TIMESTAMP
		WHERE server_id = ?`, nextConfig.MaxPlayers, serverID); err != nil {
		_ = s.rewriteRuntimeConfigFile(server.ConfigFilePath, rawConfigJSON)
		return models.Server{}, fmt.Errorf("update runtime max players: %w", err)
	}

	s.logAdminSteamIDMerge(server.ID, server.Name, strings.ToLower(strings.TrimSpace(plan.Code)), s.cfg.PuckTemplateConfig, adminMerge)

	return s.GetByID(ctx, user.ID, serverID)
}

func (s *Service) UpdateConfigAsAdmin(ctx context.Context, serverID int64, input UpdateConfigInput) (models.Server, error) {
	server, rawConfigJSON, err := s.getServerRecordAny(ctx, serverID)
	if err != nil {
		return models.Server{}, err
	}

	payload, err := parseRuntimeConfig(rawConfigJSON)
	if err != nil {
		return models.Server{}, err
	}
	templateConfig, err := s.loadTemplateConfig()
	if err != nil {
		return models.Server{}, err
	}

	currentConfig := extractServerConfig(payload)
	templateAdminSteamIDs := mandatoryTemplateAdminSteamIDs(templateConfig)
	templateBaseMods := normalizeMods(extractServerConfig(templateConfig).Mods)
	nextConfig, adminMerge, err := s.applyHostedServerConfig(payload, currentConfig, input.Config, adminControlPlan, templateAdminSteamIDs, templateBaseMods)
	if err != nil {
		return models.Server{}, err
	}

	renderedConfig, err := renderRuntimeConfig(payload)
	if err != nil {
		return models.Server{}, err
	}

	if err := s.rewriteRuntimeConfigFile(server.ConfigFilePath, renderedConfig); err != nil {
		return models.Server{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE servers
		SET desired_tick_rate = ?, max_players = ?, config_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, nextConfig.ServerTickRate, nextConfig.MaxPlayers, renderedConfig, serverID)
	if err != nil {
		_ = s.rewriteRuntimeConfigFile(server.ConfigFilePath, rawConfigJSON)
		return models.Server{}, fmt.Errorf("admin update server config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.Server{}, fmt.Errorf("admin config update rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return models.Server{}, sql.ErrNoRows
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE server_runtime
		SET max_players = ?, updated_at = CURRENT_TIMESTAMP
		WHERE server_id = ?`, nextConfig.MaxPlayers, serverID); err != nil {
		_ = s.rewriteRuntimeConfigFile(server.ConfigFilePath, rawConfigJSON)
		return models.Server{}, fmt.Errorf("admin update runtime max players: %w", err)
	}

	s.logAdminSteamIDMerge(server.ID, server.Name, strings.ToLower(strings.TrimSpace(adminControlPlan.Code)), s.cfg.PuckTemplateConfig, adminMerge)

	return s.GetAnyByID(ctx, serverID)
}

func (s *Service) DeleteServer(ctx context.Context, ownerID int64, serverID int64) error {
	server, _, err := s.getServerRecord(ctx, ownerID, serverID)
	if err != nil {
		return err
	}

	planCode, serviceName, err := s.resolveRuntimeUnit(ctx, server)
	if err != nil {
		s.logf("[server-runtime] server_id=%d server_name=%q plan_code=unknown unit=%s action=delete result=error error=%q", server.ID, server.Name, strings.TrimSpace(server.ServiceName), err.Error())
		return fmt.Errorf("%w: %s", ErrRuntimeCleanupFailed, err.Error())
	}

	if err := s.cleanupRuntimeServer(ctx, server, planCode, serviceName); err != nil {
		s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=delete result=error error=%q", server.ID, server.Name, planCode, serviceName, err.Error())
		return fmt.Errorf("%w: %s", ErrRuntimeCleanupFailed, err.Error())
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ? AND owner_id = ?`, serverID, ownerID)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete server rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=delete result=ok", server.ID, server.Name, planCode, serviceName)

	return nil
}

func (s *Service) DeleteServerAsAdmin(ctx context.Context, serverID int64) error {
	server, _, err := s.getServerRecordAny(ctx, serverID)
	if err != nil {
		return err
	}

	planCode, serviceName, err := s.resolveRuntimeUnit(ctx, server)
	if err != nil {
		s.logf("[server-runtime] server_id=%d server_name=%q plan_code=unknown unit=%s action=delete result=error error=%q", server.ID, server.Name, strings.TrimSpace(server.ServiceName), err.Error())
		return fmt.Errorf("%w: %s", ErrRuntimeCleanupFailed, err.Error())
	}

	if err := s.cleanupRuntimeServer(ctx, server, planCode, serviceName); err != nil {
		s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=delete result=error error=%q", server.ID, server.Name, planCode, serviceName, err.Error())
		return fmt.Errorf("%w: %s", ErrRuntimeCleanupFailed, err.Error())
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, serverID)
	if err != nil {
		return fmt.Errorf("admin delete server: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("admin delete server rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	s.logf("[server-runtime] server_id=%d server_name=%q plan_code=%s unit=%s action=delete result=ok", server.ID, server.Name, planCode, serviceName)

	return nil
}

func ParseStoredConfig(configJSON string) (models.ServerConfig, error) {
	payload, err := parseRuntimeConfig(configJSON)
	if err != nil {
		return models.ServerConfig{}, err
	}

	return extractServerConfig(payload), nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanServer(row scanner) (models.Server, string, error) {
	var server models.Server
	var rawConfigJSON string

	err := row.Scan(
		&server.ID,
		&server.OwnerID,
		&server.Name,
		&server.Slug,
		&server.Region,
		&server.ConfigFilePath,
		&server.ServiceName,
		&server.Status,
		&server.DesiredTickRate,
		&server.MaxPlayers,
		&server.PlayerCount,
		&server.ProcessState,
		&server.LastActionError,
		&rawConfigJSON,
	)
	if err != nil {
		return models.Server{}, "", fmt.Errorf("scan server: %w", err)
	}

	payload, err := parseRuntimeConfig(rawConfigJSON)
	if err != nil {
		return models.Server{}, "", err
	}
	server.Config = extractServerConfig(payload)

	return server, rawConfigJSON, nil
}

func uniqueSlugCandidate(name string) string {
	sanitized := strings.ToLower(strings.TrimSpace(name))
	sanitized = slugPattern.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return "puck-server"
	}

	return sanitized
}

func (s *Service) ensureUniqueSlug(ctx context.Context, baseSlug string) (string, error) {
	for index := 0; index < 100; index++ {
		candidate := baseSlug
		if index > 0 {
			candidate = fmt.Sprintf("%s-%d", baseSlug, index+1)
		}

		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM servers WHERE slug = ?`, candidate).Scan(&count); err != nil {
			return "", fmt.Errorf("check server slug: %w", err)
		}

		if count == 0 {
			return candidate, nil
		}
	}

	return "", errors.New("unable to allocate a unique server slug")
}

func nullableAttributionValue(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return value
}

func nullableAttributionTimestamp(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return value
}

func firstNonEmptyAttribution(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func (s *Service) markActionFailure(ctx context.Context, serverID int64, errorMessage string) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE servers
		SET status = 'error', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, serverID); err != nil {
		return fmt.Errorf("mark server action failure: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE server_runtime
		SET process_state = 'error', last_action_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE server_id = ?`, errorMessage, serverID); err != nil {
		return fmt.Errorf("mark runtime action failure: %w", err)
	}

	return nil
}

func (s *Service) resolveRuntimeUnit(ctx context.Context, server models.Server) (string, string, error) {
	planCode := "unknown"
	if server.OwnerID > 0 {
		plan, err := s.plans.GetForUser(ctx, server.OwnerID)
		if err == nil {
			planCode = strings.ToLower(strings.TrimSpace(plan.Code))
		} else if strings.TrimSpace(server.ServiceName) == "" {
			return "", "", fmt.Errorf("load owner plan: %w", err)
		}
	}

	serviceName := s.resolveServiceName(planCode, server.ConfigFilePath, server.ServiceName)
	if serviceName == "" {
		return "", "", fmt.Errorf("missing runtime metadata for this server")
	}

	return planCode, serviceName, nil
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}

func (s *Service) logAdminSteamIDMerge(serverID int64, serverName string, planCode string, templatePath string, merge adminSteamIDMergeResult) {
	s.logf(
		"[server-config] server_id=%d server_name=%q template_path=%s template_admin_steam_ids=%v requested_user_admin_steam_ids=%v final_merged_admin_steam_ids=%v plan_code=%s dropped_due_to_plan_limit=%t dropped_user_admin_steam_ids=%v",
		serverID,
		strings.TrimSpace(serverName),
		strings.TrimSpace(templatePath),
		merge.TemplateAdminSteamIDs,
		merge.RequestedUserAdminSteamIDs,
		merge.FinalAdminSteamIDs,
		planCode,
		merge.DroppedDueToPlanLimit,
		merge.DroppedUserAdminSteamIDs,
	)
}
