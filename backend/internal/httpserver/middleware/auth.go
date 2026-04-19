package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"speedhosting/backend/internal/models"
	authservice "speedhosting/backend/internal/services/auth"
)

type contextKey string

const userContextKey contextKey = "speedhosting.user"

func RequireAuth(service *authservice.Service, cookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}

			user, err := service.AuthenticateSession(r.Context(), cookie.Value)
			if err != nil {
				status := http.StatusInternalServerError
				message := "unable to resolve session"
				if errors.Is(err, authservice.ErrInvalidSession) {
					status = http.StatusUnauthorized
					message = "authentication required"
				}

				writeJSON(w, status, map[string]string{"error": message})
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := CurrentUser(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}

		if user.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func CurrentUser(r *http.Request) (models.AuthenticatedUser, bool) {
	user, ok := r.Context().Value(userContextKey).(models.AuthenticatedUser)
	return user, ok
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprint(w, mustMarshal(payload))
}

func mustMarshal(payload any) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return `{"error":"internal server error"}`
	}

	return string(encoded)
}
