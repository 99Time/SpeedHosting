package handlers

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"speedhosting/backend/internal/config"
	"speedhosting/backend/internal/httpserver/middleware"
	"speedhosting/backend/internal/models"
	analyticsservice "speedhosting/backend/internal/services/analytics"
	authservice "speedhosting/backend/internal/services/auth"
)

type AuthHandler struct {
	auth              *authservice.Service
	analytics         *analyticsservice.Service
	sessionCookieName string
	sessionTTL        time.Duration
	cookieSecure      bool
}

type registerRequest struct {
	DisplayName string                        `json:"displayName"`
	Email       string                        `json:"email"`
	Password    string                        `json:"password"`
	Acquisition models.AcquisitionAttribution `json:"acquisition"`
}

type loginRequest struct {
	Email       string                        `json:"email"`
	Password    string                        `json:"password"`
	Acquisition models.AcquisitionAttribution `json:"acquisition"`
}

func NewAuthHandler(auth *authservice.Service, analytics *analyticsservice.Service, cfg config.Config) *AuthHandler {
	return &AuthHandler{
		auth:              auth,
		analytics:         analytics,
		sessionCookieName: cfg.SessionCookieName,
		sessionTTL:        cfg.SessionTTL,
		cookieSecure:      cfg.CookieSecure,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var request registerRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, token, err := h.auth.Register(r.Context(), authservice.RegisterInput{
		DisplayName: request.DisplayName,
		Email:       request.Email,
		Password:    request.Password,
		Attribution: request.Acquisition,
	}, sessionMetadataFromRequest(r))
	if err != nil {
		h.writeAuthError(w, err)
		return
	}

	if h.analytics != nil {
		_ = h.analytics.TrackEvent(r.Context(), &user.ID, analyticsservice.TrackEventInput{
			Name:            "register_success",
			Source:          request.Acquisition.Source,
			Route:           firstAuthRoute(request.Acquisition.Route, "/register"),
			LandingPath:     request.Acquisition.LandingPath,
			FullURL:         request.Acquisition.FullURL,
			SessionID:       request.Acquisition.SessionID,
			ClientTimestamp: request.Acquisition.Timestamp,
		})
	}

	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, token, err := h.auth.Login(r.Context(), authservice.LoginInput{
		Email:       request.Email,
		Password:    request.Password,
		Attribution: request.Acquisition,
	}, sessionMetadataFromRequest(r))
	if err != nil {
		h.writeAuthError(w, err)
		return
	}

	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.sessionCookieName)
	if err == nil && cookie.Value != "" {
		_ = h.auth.DeleteSession(r.Context(), cookie.Value)
	}

	h.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session missing")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *AuthHandler) writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authservice.ErrValidation):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, authservice.ErrEmailAlreadyRegistered):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, authservice.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "authentication failed")
	}
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
		Expires:  time.Now().Add(h.sessionTTL),
		MaxAge:   int(h.sessionTTL.Seconds()),
	})
}

func (h *AuthHandler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func sessionMetadataFromRequest(r *http.Request) authservice.SessionMetadata {
	return authservice.SessionMetadata{
		UserAgent: strings.TrimSpace(r.UserAgent()),
		IPAddress: requestIPAddress(r),
	}
}

func requestIPAddress(r *http.Request) string {
	forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}

func firstAuthRoute(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return "/"
}
