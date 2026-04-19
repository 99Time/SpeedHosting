package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"

	puckservice "speedhosting/backend/internal/services/puck"
)

func TestMatchResultsHandlerLatestReturnsNotFoundWhenMissing(t *testing.T) {
	database := openMatchResultsHandlerTestDB(t)
	handler := NewMatchResultsHandler(puckservice.NewService(database, nil, nil))

	request := httptest.NewRequest(http.MethodGet, "/api/ranked/matches/latest", nil)
	recorder := httptest.NewRecorder()

	handler.Latest(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "ranked match result not found" {
		t.Fatalf("unexpected error message %q", payload["error"])
	}
}

func TestMatchResultsHandlerLatestReturnsLatestMatch(t *testing.T) {
	database := openMatchResultsHandlerTestDB(t)
	handler := NewMatchResultsHandler(puckservice.NewService(database, nil, nil))

	payloadJSON := `{"matchId":7,"createdAt":"2026-04-13T11:00:00Z","serverName":"Ranked Arena","winningTeam":"blue","blueScore":5,"redScore":1,"players":[{"steamId":"76561190000000041","goals":0,"assists":0,"isMvp":true}]}`
	if _, err := database.Exec(`
		INSERT INTO ranked_match_results (server_mode, winning_team, payload_json, created_at)
		VALUES (?, ?, ?, ?)`, "competitive", "blue", payloadJSON, "2026-04-13T11:00:00Z"); err != nil {
		t.Fatalf("insert ranked match result: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/ranked/matches/latest", nil)
	recorder := httptest.NewRecorder()

	handler.Latest(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var payload struct {
		Data struct {
			MatchID     int64  `json:"matchId"`
			ServerName  string `json:"serverName"`
			WinningTeam string `json:"winningTeam"`
			Players     []struct {
				IsMVP *bool `json:"isMvp"`
			} `json:"players"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.MatchID != 7 {
		t.Fatalf("expected match id 7, got %d", payload.Data.MatchID)
	}
	if payload.Data.WinningTeam != "blue" {
		t.Fatalf("expected winning team blue, got %q", payload.Data.WinningTeam)
	}
	if payload.Data.ServerName != "Ranked Arena" {
		t.Fatalf("expected server name Ranked Arena, got %q", payload.Data.ServerName)
	}
	if len(payload.Data.Players) != 1 || payload.Data.Players[0].IsMVP == nil || !*payload.Data.Players[0].IsMVP {
		t.Fatalf("expected player isMvp to be preserved %+v", payload.Data.Players)
	}
}

func openMatchResultsHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

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
