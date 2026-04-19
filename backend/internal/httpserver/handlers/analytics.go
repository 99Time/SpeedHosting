package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"speedhosting/backend/internal/models"
	analyticsservice "speedhosting/backend/internal/services/analytics"
	authservice "speedhosting/backend/internal/services/auth"
)

type AnalyticsHandler struct {
	analytics         *analyticsservice.Service
	auth              *authservice.Service
	sessionCookieName string
}

type analyticsEventRequest struct {
	Name        string                        `json:"name"`
	Source      string                        `json:"source"`
	Route       string                        `json:"route"`
	LandingPath string                        `json:"landingPath"`
	FullURL     string                        `json:"fullUrl"`
	SessionID   string                        `json:"sessionId"`
	Timestamp   string                        `json:"timestamp"`
	Metadata    map[string]any                `json:"metadata"`
	Acquisition models.AcquisitionAttribution `json:"acquisition"`
}

func NewAnalyticsHandler(analytics *analyticsservice.Service, auth *authservice.Service, sessionCookieName string) *AnalyticsHandler {
	return &AnalyticsHandler{
		analytics:         analytics,
		auth:              auth,
		sessionCookieName: sessionCookieName,
	}
}

func (h *AnalyticsHandler) Track(w http.ResponseWriter, r *http.Request) {
	var request analyticsEventRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var userID *int64
	if user, ok := h.optionalUser(r); ok {
		userID = &user.ID
	}

	input := analyticsservice.TrackEventInput{
		Name:            request.Name,
		Source:          firstNonEmpty(request.Source, request.Acquisition.Source),
		Route:           firstNonEmpty(request.Route, request.Acquisition.Route),
		LandingPath:     firstNonEmpty(request.LandingPath, request.Acquisition.LandingPath),
		FullURL:         firstNonEmpty(request.FullURL, request.Acquisition.FullURL),
		SessionID:       firstNonEmpty(request.SessionID, request.Acquisition.SessionID),
		ClientTimestamp: firstNonEmpty(request.Timestamp, request.Acquisition.Timestamp),
		Metadata:        request.Metadata,
	}

	if err := h.analytics.TrackEvent(r.Context(), userID, input); err != nil {
		writeError(w, http.StatusInternalServerError, "unable to record analytics event")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (h *AnalyticsHandler) optionalUser(r *http.Request) (models.AuthenticatedUser, bool) {
	if h.auth == nil || strings.TrimSpace(h.sessionCookieName) == "" {
		return models.AuthenticatedUser{}, false
	}

	cookie, err := r.Cookie(h.sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return models.AuthenticatedUser{}, false
	}

	user, err := h.auth.AuthenticateSession(r.Context(), cookie.Value)
	if err != nil {
		return models.AuthenticatedUser{}, false
	}

	return user, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}
