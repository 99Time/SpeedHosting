package handlers

import (
	"net/http"

	"speedhosting/backend/internal/httpserver/middleware"
	planservice "speedhosting/backend/internal/services/plan"
	serverservice "speedhosting/backend/internal/services/server"
)

type PlanHandler struct {
	plans   *planservice.Service
	servers *serverservice.Service
}

func NewPlanHandler(plans *planservice.Service, servers *serverservice.Service) *PlanHandler {
	return &PlanHandler{plans: plans, servers: servers}
}

func (h *PlanHandler) Show(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session missing")
		return
	}

	plan, err := h.plans.GetForUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	servers, err := h.servers.ListByOwner(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"plan": plan,
		"usage": map[string]int{
			"serverCount": len(servers),
		},
	})
}
