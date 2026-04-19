package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	adminservice "speedhosting/backend/internal/services/admin"
	serverservice "speedhosting/backend/internal/services/server"
)

type AdminHandler struct {
	admin   *adminservice.Service
	servers *serverservice.Service
}

func NewAdminHandler(admin *adminservice.Service, servers *serverservice.Service) *AdminHandler {
	return &AdminHandler{admin: admin, servers: servers}
}

func (h *AdminHandler) Overview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.admin.Overview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, overview)
}

func (h *AdminHandler) UpdateUserPlan(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var payload struct {
		PlanCode string `json:"planCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.admin.UpdateUserPlan(r.Context(), userID, payload.PlanCode)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, sql.ErrNoRows):
			status = http.StatusNotFound
		case errors.Is(err, adminservice.ErrPlanNotFound):
			status = http.StatusBadRequest
		}

		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *AdminHandler) ServerAction(w http.ResponseWriter, r *http.Request) {
	serverID, err := strconv.ParseInt(chi.URLParam(r, "serverID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}

	var payload struct {
		Action string `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	server, err := h.servers.PerformAdminAction(r.Context(), serverID, payload.Action)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		} else if errors.Is(err, serverservice.ErrUnsupportedAction) || errors.Is(err, serverservice.ErrInvalidServerInput) {
			status = http.StatusBadRequest
		} else if errors.Is(err, serverservice.ErrRuntimeActionFailed) {
			status = http.StatusBadGateway
		}

		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"server": server})
}

func (h *AdminHandler) UpdateServerConfig(w http.ResponseWriter, r *http.Request) {
	serverID, err := strconv.ParseInt(chi.URLParam(r, "serverID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}

	var input serverservice.UpdateConfigInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	server, err := h.servers.UpdateConfigAsAdmin(r.Context(), serverID, input)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, sql.ErrNoRows):
			status = http.StatusNotFound
		case errors.Is(err, serverservice.ErrInvalidServerInput):
			status = http.StatusBadRequest
		case errors.Is(err, serverservice.ErrPlanLimitReached):
			status = http.StatusConflict
		}

		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"server": server})
}

func (h *AdminHandler) DeleteServer(w http.ResponseWriter, r *http.Request) {
	serverID, err := strconv.ParseInt(chi.URLParam(r, "serverID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}

	if err := h.servers.DeleteServerAsAdmin(r.Context(), serverID); err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, sql.ErrNoRows):
			status = http.StatusNotFound
		case errors.Is(err, serverservice.ErrRuntimeCleanupFailed):
			status = http.StatusBadGateway
		}

		writeError(w, status, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
