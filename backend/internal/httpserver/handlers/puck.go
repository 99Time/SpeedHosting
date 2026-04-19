package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"speedhosting/backend/internal/models"
	puckservice "speedhosting/backend/internal/services/puck"
)

type PuckHandler struct {
	puck *puckservice.Service
}

func NewPuckHandler(puck *puckservice.Service) *PuckHandler {
	return &PuckHandler{puck: puck}
}

func (h *PuckHandler) GetPlayerState(w http.ResponseWriter, r *http.Request) {
	steamID := strings.TrimSpace(chi.URLParam(r, "steamID"))
	if steamID == "" {
		writeError(w, http.StatusBadRequest, "steam id is required")
		return
	}

	state, err := h.puck.GetPlayerState(r.Context(), steamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to resolve player state")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": state})
}

func (h *PuckHandler) ReportMatch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var payload json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.puck.ReportMatch(r.Context(), payload); err != nil {
		writeError(w, http.StatusInternalServerError, "unable to ingest match result")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *PuckHandler) Mute(w http.ResponseWriter, r *http.Request) {
	h.applyModerationCommand(w, r, h.puck.Mute)
}

func (h *PuckHandler) TempBan(w http.ResponseWriter, r *http.Request) {
	h.applyModerationCommand(w, r, h.puck.TempBan)
}

func (h *PuckHandler) applyModerationCommand(w http.ResponseWriter, r *http.Request, operation func(context.Context, models.PuckModerationCommand) (models.PuckModerationWriteResult, error)) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	defer r.Body.Close()

	var request models.PuckModerationCommand
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := operation(r.Context(), request)
	if err != nil {
		status := http.StatusInternalServerError
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "steam id") || strings.Contains(message, "durationseconds") || strings.Contains(message, "duration") {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
