CREATE TABLE IF NOT EXISTS plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    max_servers INTEGER NOT NULL,
    max_tick_rate INTEGER NOT NULL,
    max_admins INTEGER NOT NULL,
    allow_custom_mods INTEGER NOT NULL DEFAULT 0,
    allow_advanced_config INTEGER NOT NULL DEFAULT 0,
    auto_shutdown_when_empty INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    plan_id INTEGER NOT NULL,
    role TEXT NOT NULL DEFAULT 'customer',
    first_acquisition_source TEXT NULL,
    latest_acquisition_source TEXT NULL,
    first_acquisition_timestamp TEXT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (plan_id) REFERENCES plans(id)
);

CREATE TABLE IF NOT EXISTS servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    region TEXT NOT NULL,
    config_file_path TEXT NOT NULL,
    service_name TEXT NOT NULL,
    acquisition_source TEXT NULL,
    acquisition_timestamp TEXT NULL,
    acquisition_session_id TEXT NULL,
    status TEXT NOT NULL DEFAULT 'stopped',
    desired_tick_rate INTEGER NOT NULL,
    max_players INTEGER NOT NULL DEFAULT 10,
    auto_shutdown_enabled INTEGER NOT NULL DEFAULT 0,
    config_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS server_admins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    role TEXT NOT NULL DEFAULT 'admin',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, user_id),
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS server_runtime (
    server_id INTEGER PRIMARY KEY,
    process_state TEXT NOT NULL DEFAULT 'stopped',
    player_count INTEGER NOT NULL DEFAULT 0,
    max_players INTEGER NOT NULL DEFAULT 10,
    last_seen_at TEXT NULL,
    last_empty_at TEXT NULL,
    last_start_at TEXT NULL,
    last_stop_at TEXT NULL,
    last_action_error TEXT NULL,
    status_payload_json TEXT NULL,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS auth_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    user_agent TEXT NULL,
    ip_address TEXT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

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
);

CREATE TABLE IF NOT EXISTS discord_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    discord_id TEXT NOT NULL UNIQUE,
    steam_id TEXT NOT NULL UNIQUE,
    discord_display TEXT NULL,
    last_known_game_name TEXT NULL,
    last_known_game_player_number TEXT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

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
);

CREATE TABLE IF NOT EXISTS ranked_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    steam_id TEXT NOT NULL UNIQUE,
    display_name TEXT NULL,
    mmr INTEGER NOT NULL DEFAULT 400,
    goals INTEGER NOT NULL DEFAULT 0,
    assists INTEGER NOT NULL DEFAULT 0,
    secondary_assists INTEGER NOT NULL DEFAULT 0,
    wins INTEGER NOT NULL DEFAULT 0,
    losses INTEGER NOT NULL DEFAULT 0,
    star_points INTEGER NOT NULL DEFAULT 0,
    win_streak INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ranked_match_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_mode TEXT NOT NULL DEFAULT 'competitive',
    winning_team TEXT NULL,
    payload_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

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
);

CREATE INDEX IF NOT EXISTS idx_analytics_events_source_name ON analytics_events (source, event_name);
CREATE INDEX IF NOT EXISTS idx_analytics_events_user_name ON analytics_events (user_id, event_name);
CREATE INDEX IF NOT EXISTS idx_discord_links_steam_id ON discord_links (steam_id);
CREATE INDEX IF NOT EXISTS idx_ranked_link_sessions_discord_status ON ranked_link_sessions (discord_id, status);
CREATE INDEX IF NOT EXISTS idx_ranked_link_sessions_expires_at ON ranked_link_sessions (expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS ux_ranked_link_sessions_pending_discord ON ranked_link_sessions (discord_id) WHERE status = 'pending' AND used = 0;
CREATE INDEX IF NOT EXISTS idx_ranked_profiles_mmr ON ranked_profiles (mmr DESC, steam_id ASC);
CREATE INDEX IF NOT EXISTS idx_ranked_match_results_created_at ON ranked_match_results (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_puck_punishments_steam_active ON puck_punishments (steam_id, active, punishment_type);
CREATE INDEX IF NOT EXISTS idx_puck_punishments_expires_at ON puck_punishments (expires_at);
