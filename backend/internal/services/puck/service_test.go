package puck

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func TestReportMatchPersistsAuthoritativeStats(t *testing.T) {
	ctx := context.Background()
	database := openPuckTestDB(t)
	service := NewService(database, nil, nil)

	payload := json.RawMessage(`{
		"serverMode": "competitive",
		"players": [
			{
				"steamId": "76561190000000011",
				"displayName": "Scorer",
				"goals": 2,
				"assists": 1,
				"secondaryAssists": 1,
				"shots": 9,
				"saves": 3,
				"won": true
			},
			{
				"steamId": "76561190000000012",
				"displayName": "Playmaker",
				"goals": 0,
				"assists": 2,
				"result": "loss"
			}
		]
	}`)

	if err := service.ReportMatch(ctx, payload); err != nil {
		t.Fatalf("report match: %v", err)
	}

	var goals int
	var assists int
	var secondaryAssists int
	var wins int
	var losses int
	if err := database.QueryRowContext(ctx, `SELECT goals, assists, secondary_assists, wins, losses FROM ranked_profiles WHERE steam_id = ?`, "76561190000000011").Scan(&goals, &assists, &secondaryAssists, &wins, &losses); err != nil {
		t.Fatalf("load scorer stats: %v", err)
	}
	if goals != 2 || assists != 1 || secondaryAssists != 1 || wins != 1 || losses != 0 {
		t.Fatalf("unexpected scorer totals goals=%d assists=%d secondaryAssists=%d wins=%d losses=%d", goals, assists, secondaryAssists, wins, losses)
	}

	if err := database.QueryRowContext(ctx, `SELECT goals, assists, secondary_assists, wins, losses FROM ranked_profiles WHERE steam_id = ?`, "76561190000000012").Scan(&goals, &assists, &secondaryAssists, &wins, &losses); err != nil {
		t.Fatalf("load playmaker stats: %v", err)
	}
	if goals != 0 || assists != 2 || secondaryAssists != 0 || wins != 0 || losses != 1 {
		t.Fatalf("unexpected playmaker totals goals=%d assists=%d secondaryAssists=%d wins=%d losses=%d", goals, assists, secondaryAssists, wins, losses)
	}
}

func TestReportMatchPersistsEnrichedRecentResult(t *testing.T) {
	ctx := context.Background()
	database := openPuckTestDB(t)
	service := NewService(database, nil, nil)

	if _, err := database.ExecContext(ctx, `
		INSERT INTO discord_links (discord_id, discord_display, steam_id, last_known_game_name, last_known_game_player_number, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"123456789012345678",
		"DiscordScorer",
		"76561190000000011",
		"ScorerName",
		"0",
	); err != nil {
		t.Fatalf("insert discord link: %v", err)
	}

	payload := json.RawMessage(`{
		"serverMode": "competitive",
		"serverName": "Ranked Central #1",
		"winner": "blue",
		"summary": "Blue closed the series 4-2.",
		"blueScore": 4,
		"redScore": 2,
		"mvpSteamId": "76561190000000011",
		"players": [
			{
				"steamId": "76561190000000011",
				"displayName": "Scorer",
				"team": "blue",
				"goals": 2,
				"assists": 1,
				"secondaryAssists": 1,
				"mmrBefore": 498,
				"mmrAfter": 512,
				"won": true,
				"mvp": true
			},
			{
				"steamId": "76561190000000012",
				"displayName": "Playmaker",
				"team": "red",
				"goals": 0,
				"assists": 2,
				"mmrBefore": 430,
				"mmrAfter": 421,
				"result": "loss"
			}
		]
	}`)

	if err := service.ReportMatch(ctx, payload); err != nil {
		t.Fatalf("report match: %v", err)
	}

	results, err := service.RecentMatchResults(ctx, 10)
	if err != nil {
		t.Fatalf("recent match results: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 recent result, got %d", len(results))
	}

	result := results[0]
	if result.MatchID == 0 {
		t.Fatalf("expected stored match id")
	}
	if result.ServerName != "Ranked Central #1" {
		t.Fatalf("unexpected server name %q", result.ServerName)
	}
	if result.ServerMode != "competitive" {
		t.Fatalf("unexpected server mode %q", result.ServerMode)
	}
	if result.WinningTeam != "blue" {
		t.Fatalf("unexpected winning team %q", result.WinningTeam)
	}
	if result.BlueScore == nil || *result.BlueScore != 4 {
		t.Fatalf("unexpected blue score %+v", result.BlueScore)
	}
	if result.RedScore == nil || *result.RedScore != 2 {
		t.Fatalf("unexpected red score %+v", result.RedScore)
	}
	if result.Summary != "Blue closed the series 4-2." {
		t.Fatalf("unexpected summary %q", result.Summary)
	}
	if result.MVP == nil || result.MVP.SteamID != "76561190000000011" {
		t.Fatalf("unexpected mvp %+v", result.MVP)
	}
	if result.MVP.IsMVP == nil || !*result.MVP.IsMVP {
		t.Fatalf("expected persisted mvp flag on result mvp %+v", result.MVP)
	}
	if len(result.Players) != 2 {
		t.Fatalf("expected 2 players, got %d", len(result.Players))
	}

	player := result.Players[0]
	if player.DiscordID != "123456789012345678" {
		t.Fatalf("unexpected discord id %q", player.DiscordID)
	}
	if player.LinkedDiscordDisplay != "DiscordScorer" {
		t.Fatalf("unexpected linked discord display %q", player.LinkedDiscordDisplay)
	}
	if player.LastKnownGamePlayerNumber != "0" {
		t.Fatalf("unexpected last known game player number %q", player.LastKnownGamePlayerNumber)
	}
	if player.Tier == nil || player.Tier.TierKey != "platinum" {
		t.Fatalf("unexpected tier %+v", player.Tier)
	}
	if player.MMRDelta == nil || *player.MMRDelta != 14 {
		t.Fatalf("unexpected mmr delta %+v", player.MMRDelta)
	}
	if player.SecondaryAssists == nil || *player.SecondaryAssists != 1 {
		t.Fatalf("unexpected secondary assists %+v", player.SecondaryAssists)
	}
	if player.IsMVP == nil || !*player.IsMVP {
		t.Fatalf("expected player isMvp flag to be preserved %+v", player.IsMVP)
	}
}

func TestReportMatchIncludesZeroStatPlayersWhenNotExcludedFromMMR(t *testing.T) {
	ctx := context.Background()
	database := openPuckTestDB(t)
	service := NewService(database, nil, nil)

	payload := json.RawMessage(`{
		"serverMode": "competitive",
		"Players": [
			{
				"SteamID": "76561190000000021",
				"DisplayName": "Defender",
				"Goals": 0,
				"Assists": 0,
				"ExcludedFromMmr": false,
				"Team": "Blue",
				"Won": true
			},
			{
				"SteamID": "76561190000000022",
				"DisplayName": "Spectator",
				"Goals": 5,
				"Assists": 5,
				"ExcludedFromMmr": true,
				"Team": "Red",
				"Won": false
			}
		]
	}`)

	if err := service.ReportMatch(ctx, payload); err != nil {
		t.Fatalf("report match: %v", err)
	}

	var goals int
	var assists int
	if err := database.QueryRowContext(ctx, `SELECT goals, assists FROM ranked_profiles WHERE steam_id = ?`, "76561190000000021").Scan(&goals, &assists); err != nil {
		t.Fatalf("load defender stats: %v", err)
	}
	if goals != 0 || assists != 0 {
		t.Fatalf("unexpected defender totals goals=%d assists=%d", goals, assists)
	}

	var excludedCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM ranked_profiles WHERE steam_id = ?`, "76561190000000022").Scan(&excludedCount); err != nil {
		t.Fatalf("load excluded player count: %v", err)
	}
	if excludedCount != 0 {
		t.Fatalf("expected excluded player to be skipped, got count=%d", excludedCount)
	}

	results, err := service.RecentMatchResults(ctx, 10)
	if err != nil {
		t.Fatalf("recent match results: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 recent result, got %d", len(results))
	}
	if len(results[0].Players) != 1 {
		t.Fatalf("expected only the non-excluded player in recent result, got %d", len(results[0].Players))
	}
	if results[0].Players[0].SteamID != "76561190000000021" {
		t.Fatalf("unexpected persisted player %q", results[0].Players[0].SteamID)
	}
	if results[0].Players[0].Goals != 0 || results[0].Players[0].Assists != 0 {
		t.Fatalf("expected zero-stat player to be preserved, got goals=%d assists=%d", results[0].Players[0].Goals, results[0].Players[0].Assists)
	}
}

func TestReportMatchPublicModeSkipsOfficialProgressionAndPersistence(t *testing.T) {
	ctx := context.Background()
	database := openPuckTestDB(t)
	service := NewService(database, nil, nil)

	payload := json.RawMessage(`{
		"serverMode": "public",
		"serverName": "Public Arena",
		"players": [
			{
				"steamId": "76561190000000051",
				"displayName": "Casual",
				"goals": 3,
				"assists": 1,
				"won": true
			}
		]
	}`)

	if err := service.ReportMatch(ctx, payload); err != nil {
		t.Fatalf("report match: %v", err)
	}

	var profileCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM ranked_profiles WHERE steam_id = ?`, "76561190000000051").Scan(&profileCount); err != nil {
		t.Fatalf("count public ranked profile rows: %v", err)
	}
	if profileCount != 0 {
		t.Fatalf("expected public match to skip ranked profile writes, got %d rows", profileCount)
	}

	results, err := service.RecentMatchResults(ctx, 10)
	if err != nil {
		t.Fatalf("recent match results: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected public match to skip official result persistence, got %d results", len(results))
	}
}

func TestLatestMatchResultIgnoresPublicRows(t *testing.T) {
	ctx := context.Background()
	database := openPuckTestDB(t)
	service := NewService(database, nil, nil)

	publicPayload := `{"matchId":1,"createdAt":"2026-04-13T10:00:00Z","serverName":"Public Arena","serverMode":"public","winningTeam":"blue","players":[{"steamId":"76561190000000061","goals":0,"assists":0,"isMvp":false}]}`
	competitivePayload := `{"matchId":2,"createdAt":"2026-04-13T10:05:00Z","serverName":"Ranked Beta","serverMode":"competitive","winningTeam":"red","players":[{"steamId":"76561190000000062","goals":1,"assists":2,"isMvp":true}]}`

	if _, err := database.ExecContext(ctx, `
		INSERT INTO ranked_match_results (server_mode, winning_team, payload_json, created_at)
		VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		"public", "blue", publicPayload, "2026-04-13T10:00:00Z",
		"competitive", "red", competitivePayload, "2026-04-13T10:05:00Z",
	); err != nil {
		t.Fatalf("insert ranked match results: %v", err)
	}

	result, err := service.LatestMatchResult(ctx)
	if err != nil {
		t.Fatalf("latest match result: %v", err)
	}
	if result.MatchID != 2 {
		t.Fatalf("expected latest competitive match id 2, got %d", result.MatchID)
	}
	if result.ServerMode != "competitive" {
		t.Fatalf("expected latest competitive server mode, got %q", result.ServerMode)
	}
}

func TestLatestMatchResultReturnsNewestPersistedMatch(t *testing.T) {
	ctx := context.Background()
	database := openPuckTestDB(t)
	service := NewService(database, nil, nil)

	older := `{"matchId":1,"createdAt":"2026-04-13T10:00:00Z","serverName":"Ranked Alpha","serverMode":"competitive","winningTeam":"blue","blueScore":3,"redScore":2,"players":[{"steamId":"76561190000000031","goals":0,"assists":0,"isMvp":false}]}`
	newer := `{"matchId":2,"createdAt":"2026-04-13T10:05:00Z","serverName":"Ranked Beta","serverMode":"competitive","winningTeam":"red","blueScore":1,"redScore":4,"players":[{"steamId":"76561190000000032","goals":1,"assists":2,"isMvp":true}]}`

	if _, err := database.ExecContext(ctx, `
		INSERT INTO ranked_match_results (server_mode, winning_team, payload_json, created_at)
		VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		"competitive", "blue", older, "2026-04-13T10:00:00Z",
		"competitive", "red", newer, "2026-04-13T10:05:00Z",
	); err != nil {
		t.Fatalf("insert ranked match results: %v", err)
	}

	result, err := service.LatestMatchResult(ctx)
	if err != nil {
		t.Fatalf("latest match result: %v", err)
	}
	if result.MatchID != 2 {
		t.Fatalf("expected latest match id 2, got %d", result.MatchID)
	}
	if result.WinningTeam != "red" {
		t.Fatalf("expected winning team red, got %q", result.WinningTeam)
	}
	if result.ServerName != "Ranked Beta" {
		t.Fatalf("expected server name Ranked Beta, got %q", result.ServerName)
	}
	if len(result.Players) != 1 || result.Players[0].IsMVP == nil || !*result.Players[0].IsMVP {
		t.Fatalf("expected latest persisted player isMvp to be preserved %+v", result.Players)
	}
}

func TestLatestMatchResultReturnsNotFoundWhenEmpty(t *testing.T) {
	ctx := context.Background()
	database := openPuckTestDB(t)
	service := NewService(database, nil, nil)

	_, err := service.LatestMatchResult(ctx)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func openPuckTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if _, err := database.Exec(`
		CREATE TABLE ranked_profiles (
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
		)`); err != nil {
		t.Fatalf("create ranked_profiles table: %v", err)
	}

	if _, err := database.Exec(`
		CREATE TABLE discord_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			discord_id TEXT NOT NULL,
			discord_display TEXT NULL,
			steam_id TEXT NOT NULL UNIQUE,
			last_known_game_name TEXT NULL,
			last_known_game_player_number TEXT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		t.Fatalf("create discord_links table: %v", err)
	}

	if _, err := database.Exec(`
		CREATE TABLE ranked_match_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			server_mode TEXT NOT NULL DEFAULT 'competitive',
			winning_team TEXT NULL,
			payload_json TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		t.Fatalf("create ranked_match_results table: %v", err)
	}

	return database
}
