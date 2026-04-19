package handlers

import (
	"encoding/json"
	"net/http"

	"speedhosting/backend/internal/models"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func sanitizeServerForUser(user models.AuthenticatedUser, server models.Server) models.Server {
	if user.Role == "admin" {
		return server
	}

	server.ConfigFilePath = ""
	server.ServiceName = ""
	return server
}

func sanitizeServersForUser(user models.AuthenticatedUser, servers []models.Server) []models.Server {
	sanitized := make([]models.Server, 0, len(servers))
	for _, server := range servers {
		sanitized = append(sanitized, sanitizeServerForUser(user, server))
	}

	return sanitized
}
