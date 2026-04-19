package handlers

import (
	"net/http"

	"speedhosting/backend/internal/httpserver/middleware"
	dashboardservice "speedhosting/backend/internal/services/dashboard"
)

type DashboardHandler struct {
	dashboard *dashboardservice.Service
}

func NewDashboardHandler(dashboard *dashboardservice.Service) *DashboardHandler {
	return &DashboardHandler{dashboard: dashboard}
}

func (h *DashboardHandler) Overview(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session missing")
		return
	}

	overview, err := h.dashboard.GetOverview(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	overview.Servers = sanitizeServersForUser(user, overview.Servers)

	writeJSON(w, http.StatusOK, overview)
}
