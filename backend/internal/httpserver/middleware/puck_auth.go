package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func RequirePuckAPIKey(expectedAPIKey string) func(http.Handler) http.Handler {
	return RequireBearerAPIKey(expectedAPIKey)
}

func RequireBearerAPIKey(expectedAPIKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerTokenFromHeader(r.Header.Get("Authorization"))
			if !ok || !apiKeyMatches(expectedAPIKey, token) {
				writePuckJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid api key"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func bearerTokenFromHeader(headerValue string) (string, bool) {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return "", false
	}

	parts := strings.Fields(headerValue)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	if strings.TrimSpace(parts[1]) == "" {
		return "", false
	}

	return parts[1], true
}

func apiKeyMatches(expectedAPIKey string, receivedAPIKey string) bool {
	if strings.TrimSpace(expectedAPIKey) == "" || strings.TrimSpace(receivedAPIKey) == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(expectedAPIKey), []byte(receivedAPIKey)) == 1
}

func writePuckJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprint(w, mustMarshalPuck(payload))
}

func mustMarshalPuck(payload any) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return `{"error":"internal server error"}`
	}

	return string(encoded)
}
