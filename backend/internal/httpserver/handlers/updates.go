package handlers

import (
	"net/http"

	"speedhosting/backend/internal/models"
	updatesservice "speedhosting/backend/internal/services/updates"
)

type UpdatesHandler struct {
	updates *updatesservice.Service
}

func NewUpdatesHandler(updates *updatesservice.Service) *UpdatesHandler {
	return &UpdatesHandler{updates: updates}
}

func (h *UpdatesHandler) List(w http.ResponseWriter, r *http.Request) {
	entries, err := h.updates.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to load updates")
		return
	}

	writeJSON(w, http.StatusOK, models.UpdatesResponse{Updates: entries})
}
