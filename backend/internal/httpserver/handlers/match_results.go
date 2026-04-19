package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"speedhosting/backend/internal/models"
	puckservice "speedhosting/backend/internal/services/puck"
)

type MatchResultsHandler struct {
	puck *puckservice.Service
}

func NewMatchResultsHandler(puck *puckservice.Service) *MatchResultsHandler {
	return &MatchResultsHandler{puck: puck}
}

func (h *MatchResultsHandler) Recent(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsedLimit
	}

	results, err := h.puck.RecentMatchResults(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to load recent match results")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": models.RankedMatchResultsResponse{Results: results},
	})
}

func (h *MatchResultsHandler) Latest(w http.ResponseWriter, r *http.Request) {
	result, err := h.puck.LatestMatchResult(r.Context())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "ranked match result not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "unable to load latest match result")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": result,
	})
}
