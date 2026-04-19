package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"speedhosting/backend/internal/httpserver/middleware"
	serverservice "speedhosting/backend/internal/services/server"
)

type ServerHandler struct {
	servers *serverservice.Service
}

func NewServerHandler(servers *serverservice.Service) *ServerHandler {
	return &ServerHandler{servers: servers}
}

func (h *ServerHandler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session missing")
		return
	}

	servers, err := h.servers.ListByOwner(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"servers": sanitizeServersForUser(user, servers)})
}

func (h *ServerHandler) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session missing")
		return
	}

	serverID, err := strconv.ParseInt(chi.URLParam(r, "serverID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}

	server, err := h.servers.GetByID(r.Context(), user.ID, serverID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"server": sanitizeServerForUser(user, server)})
}

func (h *ServerHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session missing")
		return
	}

	var input serverservice.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	server, err := h.servers.CreateServer(r.Context(), user, input)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, serverservice.ErrInvalidServerInput):
			status = http.StatusBadRequest
		case errors.Is(err, serverservice.ErrPlanLimitReached), errors.Is(err, serverservice.ErrDuplicateServerFile):
			status = http.StatusConflict
		case errors.Is(err, serverservice.ErrRuntimeTemplate):
			status = http.StatusFailedDependency
		}

		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"server": sanitizeServerForUser(user, server)})
}

func (h *ServerHandler) Action(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session missing")
		return
	}

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

	server, err := h.servers.PerformAction(r.Context(), user.ID, serverID, payload.Action)
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

	writeJSON(w, http.StatusOK, map[string]any{"server": sanitizeServerForUser(user, server)})
}

func (h *ServerHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session missing")
		return
	}

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

	server, err := h.servers.UpdateConfig(r.Context(), user, serverID, input)
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

	writeJSON(w, http.StatusOK, map[string]any{"server": sanitizeServerForUser(user, server)})
}

func (h *ServerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session missing")
		return
	}

	serverID, err := strconv.ParseInt(chi.URLParam(r, "serverID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}

	if err := h.servers.DeleteServer(r.Context(), user.ID, serverID); err != nil {
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
