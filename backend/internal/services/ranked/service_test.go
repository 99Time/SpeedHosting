package ranked

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"speedhosting/backend/internal/ranktier"
)

func TestSanitizeGamePlayerNumberPreservesZero(t *testing.T) {
	if got := sanitizeGamePlayerNumber("0"); got != "0" {
		t.Fatalf("expected 0 to be preserved, got %q", got)
	}
}

func TestNullableStringPreservesZero(t *testing.T) {
	value := nullableString("0")
	stringValue, ok := value.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", value)
	}
	if stringValue != "0" {
		t.Fatalf("expected 0 to be preserved, got %q", stringValue)
	}
}

func TestInitializeRankedProfileTxCreatesDefaultProfile(t *testing.T) {
	ctx := context.Background()
	database := openRankedTestDB(t)

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	initialized, err := initializeRankedProfileTx(ctx, tx, "76561190000000000", "New Verified")
	if err != nil {
		t.Fatalf("initialize ranked profile: %v", err)
	}
	if !initialized {
		t.Fatalf("expected ranked profile to be initialized")
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	var mmr int
	var wins int
	var losses int
	var starPoints int
	var winStreak int
	if err := database.QueryRowContext(ctx, `SELECT mmr, wins, losses, star_points, win_streak FROM ranked_profiles WHERE steam_id = ?`, "76561190000000000").Scan(&mmr, &wins, &losses, &starPoints, &winStreak); err != nil {
		t.Fatalf("load ranked profile: %v", err)
	}

	if mmr != ranktier.InitialMMR {
		t.Fatalf("expected mmr %d, got %d", ranktier.InitialMMR, mmr)
	}
	if wins != 0 || losses != 0 || starPoints != 0 || winStreak != 0 {
		t.Fatalf("expected zeroed ranked stats, got wins=%d losses=%d starPoints=%d winStreak=%d", wins, losses, starPoints, winStreak)
	}
}

func TestMergePersistedProfilesAddsMissingProfile(t *testing.T) {
	ctx := context.Background()
	database := openRankedTestDB(t)

	if _, err := database.ExecContext(ctx, `
		INSERT INTO ranked_profiles (steam_id, display_name, mmr, goals, assists, secondary_assists, wins, losses, star_points, win_streak)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "76561190000000001", "Fresh Player", ranktier.InitialMMR, 2, 3, 1, 0, 0, 0, 0); err != nil {
		t.Fatalf("insert ranked profile: %v", err)
	}

	service := NewService(database, nil, "", "", "", "", 0)
	merged, err := service.mergePersistedProfiles(ctx, nil)
	if err != nil {
		t.Fatalf("merge persisted profiles: %v", err)
	}

	if len(merged) != 1 {
		t.Fatalf("expected 1 merged profile, got %d", len(merged))
	}
	if merged[0].SteamID != "76561190000000001" {
		t.Fatalf("unexpected steam id %q", merged[0].SteamID)
	}
	if merged[0].MMR != ranktier.InitialMMR {
		t.Fatalf("expected mmr %d, got %d", ranktier.InitialMMR, merged[0].MMR)
	}
	if merged[0].Goals != 2 || merged[0].Assists != 3 || merged[0].SecondaryAssists != 1 {
		t.Fatalf("unexpected authoritative stats goals=%d assists=%d secondaryAssists=%d", merged[0].Goals, merged[0].Assists, merged[0].SecondaryAssists)
	}
}

func openRankedTestDB(t *testing.T) *sql.DB {
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

	return database
}
