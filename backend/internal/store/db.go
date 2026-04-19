package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"speedhosting/backend/internal/config"
	"speedhosting/backend/internal/planrules"

	_ "modernc.org/sqlite"
)

//go:embed migrations/0001_init.sql
var migrations embed.FS

func Initialize(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	if err := ensureDatabaseDirectory(cfg.DatabasePath); err != nil {
		return nil, err
	}

	database, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	database.SetMaxOpenConns(1)

	if _, err := database.ExecContext(ctx, "PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := database.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := migrate(ctx, database); err != nil {
		return nil, err
	}

	if err := ensureSchemaUpgrades(ctx, database); err != nil {
		return nil, err
	}

	if err := seed(ctx, database); err != nil {
		return nil, err
	}

	return database, nil
}

func migrate(ctx context.Context, database *sql.DB) error {
	schema, err := migrations.ReadFile("migrations/0001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}

	if _, err := database.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}

	return nil
}

func seed(ctx context.Context, database *sql.DB) error {
	statements := []string{
		`DELETE FROM auth_sessions
		 WHERE user_id IN (SELECT id FROM users WHERE email = 'demo@speedhosting.local');`,
		`DELETE FROM server_admins
		 WHERE user_id IN (SELECT id FROM users WHERE email = 'demo@speedhosting.local');`,
		`DELETE FROM server_runtime
		 WHERE server_id IN (
		   SELECT id FROM servers WHERE owner_id IN (SELECT id FROM users WHERE email = 'demo@speedhosting.local')
		 );`,
		`DELETE FROM servers
		 WHERE owner_id IN (SELECT id FROM users WHERE email = 'demo@speedhosting.local');`,
		`DELETE FROM users WHERE email = 'demo@speedhosting.local';`,
		`UPDATE servers
		 SET region = 'Nuremberg', updated_at = CURRENT_TIMESTAMP
		 WHERE region <> 'Nuremberg';`,
		`UPDATE servers
		 SET auto_shutdown_enabled = 0, updated_at = CURRENT_TIMESTAMP
		 WHERE auto_shutdown_enabled <> 0;`,
		`UPDATE users
		 SET role = 'customer', updated_at = CURRENT_TIMESTAMP
		 WHERE COALESCE(role, '') = '';`,
		`UPDATE users
		 SET role = 'admin', updated_at = CURRENT_TIMESTAMP
		 WHERE id = (
		   SELECT id FROM users ORDER BY created_at ASC, id ASC LIMIT 1
		 )
		 AND NOT EXISTS (SELECT 1 FROM users WHERE role = 'admin');`,
	}

	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("seed statement failed: %w", err)
		}
	}

	for _, capability := range planrules.PublicCatalog() {
		if _, err := database.ExecContext(ctx, `
			INSERT INTO plans (code, name, max_servers, max_tick_rate, max_admins, allow_custom_mods, allow_advanced_config, auto_shutdown_when_empty)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0)
			ON CONFLICT(code) DO UPDATE SET
			  name = excluded.name,
			  max_servers = excluded.max_servers,
			  max_tick_rate = excluded.max_tick_rate,
			  max_admins = excluded.max_admins,
			  allow_custom_mods = excluded.allow_custom_mods,
			  allow_advanced_config = excluded.allow_advanced_config,
			  auto_shutdown_when_empty = excluded.auto_shutdown_when_empty`,
			capability.Code,
			capability.Name,
			capability.MaxServers,
			capability.MaxTickRate,
			capability.MaxAdminSteamIDs,
			boolToInt(capability.AllowCustomMods),
			boolToInt(capability.AdvancedSettingsUnlocked),
		); err != nil {
			return fmt.Errorf("seed plan %s failed: %w", capability.Code, err)
		}
	}

	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ensureSchemaUpgrades(ctx context.Context, database *sql.DB) error {
	if err := ensureColumn(ctx, database, "users", "role", "TEXT NOT NULL DEFAULT 'customer'"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "users", "first_acquisition_source", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "users", "latest_acquisition_source", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "users", "first_acquisition_timestamp", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "servers", "config_file_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "servers", "service_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "servers", "acquisition_source", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "servers", "acquisition_timestamp", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "servers", "acquisition_session_id", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "server_runtime", "last_action_error", "TEXT NULL"); err != nil {
		return err
	}

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS analytics_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_name TEXT NOT NULL,
			source TEXT NOT NULL,
			route TEXT NOT NULL,
			landing_path TEXT NULL,
			full_url TEXT NULL,
			session_id TEXT NULL,
			user_id INTEGER NULL,
			server_id INTEGER NULL,
			client_timestamp TEXT NULL,
			metadata_json TEXT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
			FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE SET NULL
		)`); err != nil {
		return fmt.Errorf("ensure analytics_events table: %w", err)
	}

	if _, err := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_analytics_events_source_name ON analytics_events (source, event_name)`); err != nil {
		return fmt.Errorf("ensure analytics source index: %w", err)
	}

	if _, err := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_analytics_events_user_name ON analytics_events (user_id, event_name)`); err != nil {
		return fmt.Errorf("ensure analytics user index: %w", err)
	}

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS discord_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			discord_id TEXT NOT NULL UNIQUE,
			steam_id TEXT NOT NULL UNIQUE,
			discord_display TEXT NULL,
			last_known_game_name TEXT NULL,
			last_known_game_player_number TEXT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("ensure discord_links table: %w", err)
	}

	if err := ensureColumn(ctx, database, "discord_links", "discord_display", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "discord_links", "last_known_game_name", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "discord_links", "last_known_game_player_number", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "discord_links", "created_at", "TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "discord_links", "updated_at", "TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}

	if _, err := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_discord_links_steam_id ON discord_links (steam_id)`); err != nil {
		return fmt.Errorf("ensure discord_links steam index: %w", err)
	}

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ranked_link_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			discord_id TEXT NOT NULL,
			guild_id TEXT NULL,
			channel_id TEXT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			completed_at TEXT NULL,
			used INTEGER NOT NULL DEFAULT 0,
			steam_id TEXT NULL
		)`); err != nil {
		return fmt.Errorf("ensure ranked_link_sessions table: %w", err)
	}

	if err := ensureColumn(ctx, database, "ranked_link_sessions", "guild_id", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_link_sessions", "channel_id", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_link_sessions", "status", "TEXT NOT NULL DEFAULT 'pending'"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_link_sessions", "created_at", "TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_link_sessions", "expires_at", "TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_link_sessions", "completed_at", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_link_sessions", "used", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_link_sessions", "steam_id", "TEXT NULL"); err != nil {
		return err
	}

	if _, err := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_ranked_link_sessions_discord_status ON ranked_link_sessions (discord_id, status)`); err != nil {
		return fmt.Errorf("ensure ranked_link_sessions discord index: %w", err)
	}

	if _, err := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_ranked_link_sessions_expires_at ON ranked_link_sessions (expires_at)`); err != nil {
		return fmt.Errorf("ensure ranked_link_sessions expires index: %w", err)
	}

	if _, err := database.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS ux_ranked_link_sessions_pending_discord ON ranked_link_sessions (discord_id) WHERE status = 'pending' AND used = 0`); err != nil {
		return fmt.Errorf("ensure ranked_link_sessions pending discord unique index: %w", err)
	}

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ranked_profiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			steam_id TEXT NOT NULL UNIQUE,
			display_name TEXT NULL,
			mmr INTEGER NOT NULL DEFAULT 400,
			wins INTEGER NOT NULL DEFAULT 0,
			losses INTEGER NOT NULL DEFAULT 0,
			star_points INTEGER NOT NULL DEFAULT 0,
			win_streak INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("ensure ranked_profiles table: %w", err)
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "display_name", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "mmr", "INTEGER NOT NULL DEFAULT 400"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "goals", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "assists", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "secondary_assists", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "wins", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "losses", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "star_points", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "win_streak", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "created_at", "TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_profiles", "updated_at", "TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}

	if _, err := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_ranked_profiles_mmr ON ranked_profiles (mmr DESC, steam_id ASC)`); err != nil {
		return fmt.Errorf("ensure ranked_profiles mmr index: %w", err)
	}

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ranked_match_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			winning_team TEXT NULL,
			payload_json TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("ensure ranked_match_results table: %w", err)
	}

	if err := ensureColumn(ctx, database, "ranked_match_results", "winning_team", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_match_results", "payload_json", "TEXT NOT NULL DEFAULT '{}' "); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_match_results", "created_at", "TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "ranked_match_results", "server_mode", "TEXT NOT NULL DEFAULT 'competitive'"); err != nil {
		return err
	}

	if _, err := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_ranked_match_results_created_at ON ranked_match_results (created_at DESC, id DESC)`); err != nil {
		return fmt.Errorf("ensure ranked_match_results created index: %w", err)
	}

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS puck_punishments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			steam_id TEXT NOT NULL,
			punishment_type TEXT NOT NULL,
			reason TEXT NULL,
			issued_by TEXT NULL,
			source TEXT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT NULL,
			active INTEGER NOT NULL DEFAULT 1
		)`); err != nil {
		return fmt.Errorf("ensure puck_punishments table: %w", err)
	}

	if err := ensureColumn(ctx, database, "puck_punishments", "reason", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "puck_punishments", "issued_by", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "puck_punishments", "source", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "puck_punishments", "created_at", "TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "puck_punishments", "expires_at", "TEXT NULL"); err != nil {
		return err
	}

	if err := ensureColumn(ctx, database, "puck_punishments", "active", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}

	if _, err := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_puck_punishments_steam_active ON puck_punishments (steam_id, active, punishment_type)`); err != nil {
		return fmt.Errorf("ensure puck_punishments active index: %w", err)
	}

	if _, err := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_puck_punishments_expires_at ON puck_punishments (expires_at)`); err != nil {
		return fmt.Errorf("ensure puck_punishments expires index: %w", err)
	}

	return nil
}

func ensureColumn(ctx context.Context, database *sql.DB, tableName string, columnName string, definition string) error {
	rows, err := database.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return fmt.Errorf("inspect %s schema: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan %s schema: %w", tableName, err)
		}

		if name == columnName {
			return nil
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s schema: %w", tableName, err)
	}

	statement := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, definition)
	if _, err := database.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("add %s.%s: %w", tableName, columnName, err)
	}

	return nil
}

func ensureDatabaseDirectory(databasePath string) error {
	directory := filepath.Dir(databasePath)
	if directory == "." || directory == "" {
		return nil
	}

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	return nil
}
