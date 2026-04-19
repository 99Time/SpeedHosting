package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	rankedservice "speedhosting/backend/internal/services/ranked"
)

type RankedHandler struct {
	ranked *rankedservice.Service
}

func NewRankedHandler(ranked *rankedservice.Service) *RankedHandler {
	return &RankedHandler{ranked: ranked}
}

type linkSteamRequest struct {
	DiscordID      string `json:"discordId"`
	SteamID        string `json:"steamId"`
	DiscordDisplay string `json:"discordDisplay"`
}

type requestLinkRequest struct {
	DiscordID string `json:"discordId"`
	GuildID   string `json:"guildId"`
	ChannelID string `json:"channelId"`
}

type completeLinkRequest struct {
	SteamID          string               `json:"steamId"`
	Code             string               `json:"code"`
	GameDisplayName  string               `json:"gameDisplayName"`
	GamePlayerNumber flexibleStringNumber `json:"gamePlayerNumber"`
}

type flexibleStringNumber struct {
	Present bool
	Raw     string
	Value   string
}

func (value *flexibleStringNumber) UnmarshalJSON(data []byte) error {
	value.Present = true
	value.Raw = string(data)

	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		value.Value = ""
		return nil
	}

	var stringValue string
	if err := json.Unmarshal(trimmed, &stringValue); err == nil {
		value.Value = stringValue
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var numericValue json.Number
	if err := decoder.Decode(&numericValue); err == nil {
		value.Value = numericValue.String()
		return nil
	}

	return fmt.Errorf("gamePlayerNumber must be a string, number, or null")
}

func (h *RankedHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}

	entries, err := h.ranked.Leaderboard(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to load ranked leaderboard")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (h *RankedHandler) Rank(w http.ResponseWriter, r *http.Request) {
	steamID := strings.TrimSpace(r.URL.Query().Get("steamId"))
	discordID := strings.TrimSpace(r.URL.Query().Get("discordId"))
	query := strings.TrimSpace(r.URL.Query().Get("query"))

	if steamID == "" && discordID == "" && query == "" {
		writeError(w, http.StatusBadRequest, "steamId, discordId, or query is required")
		return
	}

	player, err := h.ranked.Rank(r.Context(), steamID, discordID, query)
	if err != nil {
		if rankedservice.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "ranked player not found")
			return
		}

		writeError(w, http.StatusInternalServerError, "unable to load ranked player")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"player": player})
}

func (h *RankedHandler) LinkSteam(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	defer r.Body.Close()

	var request linkSteamRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.ranked.LinkSteam(r.Context(), request.DiscordID, request.SteamID, request.DiscordDisplay); err != nil {
		message := err.Error()
		status := http.StatusInternalServerError
		if strings.Contains(message, "discord id") || strings.Contains(message, "steam id") {
			status = http.StatusBadRequest
		}
		writeError(w, status, message)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *RankedHandler) RequestLink(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	defer r.Body.Close()

	var request requestLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.ranked.RequestLink(r.Context(), request.DiscordID, request.GuildID, request.ChannelID)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, rankedservice.ErrDiscordAlreadyLinked):
			status = http.StatusConflict
		case strings.Contains(strings.ToLower(err.Error()), "discord id"):
			status = http.StatusBadRequest
		}

		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *RankedHandler) LinkStatus(w http.ResponseWriter, r *http.Request) {
	discordID := strings.TrimSpace(r.URL.Query().Get("discordId"))
	if discordID == "" {
		writeError(w, http.StatusBadRequest, "discordId is required")
		return
	}

	statusPayload, err := h.ranked.LinkStatus(r.Context(), discordID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToLower(err.Error()), "discord id") {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, statusPayload)
}

func (h *RankedHandler) CompleteLink(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	defer r.Body.Close()

	var request completeLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.ranked.CompleteLink(r.Context(), request.SteamID, request.Code, request.GameDisplayName, request.GamePlayerNumber.Raw, request.GamePlayerNumber.Value)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, rankedservice.ErrInvalidLinkCode):
			status = http.StatusNotFound
		case errors.Is(err, rankedservice.ErrLinkCodeExpired), errors.Is(err, rankedservice.ErrLinkCodeAlreadyUsed):
			status = http.StatusConflict
		case strings.Contains(strings.ToLower(err.Error()), "steam id"):
			status = http.StatusBadRequest
		}

		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
