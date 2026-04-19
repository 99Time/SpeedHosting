package config

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv             string
	HTTPAddress        string
	DatabasePath       string
	FrontendOrigin     string
	SessionCookieName  string
	SessionTTL         time.Duration
	CookieSecure       bool
	RankedDataPath     string
	RankedMMRPath      string
	RankedStarsPath    string
	RankedLinkAPIKey   string
	RankedLinkCodeTTL  time.Duration
	UpdatesPath        string
	PuckAPIKey         string
	PuckConfigDir      string
	PuckTemplateConfig string
	PuckSystemctlPath  string
	PuckServicePrefix  string
	PuckBasePort       int
	PuckReservedPorts  []int
}

func Load() Config {
	appEnv := getEnv("SPEEDHOSTING_ENV", "development")
	puckConfigDir := getEnv("SPEEDHOSTING_PUCK_CONFIG_DIR", "/srv/puckserver")

	return Config{
		AppEnv:             appEnv,
		HTTPAddress:        getEnv("SPEEDHOSTING_HTTP_ADDR", ":8081"),
		DatabasePath:       resolveConfiguredPath(getEnv("SPEEDHOSTING_DB_PATH", "speedhosting.db")),
		FrontendOrigin:     getEnv("SPEEDHOSTING_FRONTEND_ORIGIN", "http://localhost:5173"),
		SessionCookieName:  getEnv("SPEEDHOSTING_SESSION_COOKIE_NAME", "speedhosting_session"),
		SessionTTL:         getDurationEnv("SPEEDHOSTING_SESSION_TTL", 7*24*time.Hour),
		CookieSecure:       getBoolEnv("SPEEDHOSTING_COOKIE_SECURE", appEnv != "development"),
		RankedDataPath:     getEnv("SPEEDHOSTING_RANKED_DATA_PATH", ""),
		RankedMMRPath:      getEnv("SPEEDHOSTING_RANKED_MMR_PATH", ""),
		RankedStarsPath:    getEnv("SPEEDHOSTING_RANKED_STARS_PATH", ""),
		RankedLinkAPIKey:   getEnv("SPEEDHOSTING_RANKED_LINK_API_KEY", getEnv("API_KEY", "")),
		RankedLinkCodeTTL:  getDurationEnv("SPEEDHOSTING_RANKED_LINK_CODE_TTL", 10*time.Minute),
		UpdatesPath:        resolveConfiguredPath(getEnv("SPEEDHOSTING_UPDATES_PATH", "updates.json")),
		PuckAPIKey:         getEnv("API_KEY", ""),
		PuckConfigDir:      puckConfigDir,
		PuckTemplateConfig: getEnv("SPEEDHOSTING_PUCK_TEMPLATE_CONFIG", filepath.Join(puckConfigDir, "server_template.json")),
		PuckSystemctlPath:  getEnv("SPEEDHOSTING_PUCK_SYSTEMCTL_PATH", "systemctl"),
		PuckServicePrefix:  getEnv("SPEEDHOSTING_PUCK_SERVICE_PREFIX", "puck@"),
		PuckBasePort:       getIntEnv("SPEEDHOSTING_PUCK_BASE_PORT", 7777),
		PuckReservedPorts:  getReservedPortsEnv("SPEEDHOSTING_PUCK_RESERVED_PORTS", []int{7777, 7778, 7779, 7780, 7781, 7782, 7783, 7784, 7785, 7786, 7787, 7788, 7789, 7790, 7791, 7792, 7793, 7794, 7795, 7796, 7797, 7798, 7799}),
	}
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func resolveConfiguredPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == ":memory:" || strings.HasPrefix(value, "file:") {
		return value
	}

	resolved, err := filepath.Abs(value)
	if err != nil {
		return value
	}

	return resolved
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		parsed, err := time.ParseDuration(value)
		if err == nil {
			return parsed
		}
	}

	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		switch value {
		case "1", "true", "TRUE", "True", "yes", "YES", "on", "ON":
			return true
		case "0", "false", "FALSE", "False", "no", "NO", "off", "OFF":
			return false
		}
	}

	return fallback
}

func getIntEnv(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}

	return fallback
}

func getReservedPortsEnv(key string, fallback []int) []int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]int(nil), fallback...)
	}

	ports := make(map[int]struct{})
	for _, part := range strings.Split(value, ",") {
		segment := strings.TrimSpace(part)
		if segment == "" {
			continue
		}

		if strings.Contains(segment, "-") {
			bounds := strings.SplitN(segment, "-", 2)
			if len(bounds) != 2 {
				continue
			}

			start, startErr := strconv.Atoi(strings.TrimSpace(bounds[0]))
			end, endErr := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if startErr != nil || endErr != nil || end < start {
				continue
			}

			for port := start; port <= end; port++ {
				ports[port] = struct{}{}
			}
			continue
		}

		parsed, err := strconv.Atoi(segment)
		if err == nil {
			ports[parsed] = struct{}{}
		}
	}

	if len(ports) == 0 {
		return append([]int(nil), fallback...)
	}

	result := make([]int, 0, len(ports))
	for port := range ports {
		result = append(result, port)
	}
	sort.Ints(result)

	return result
}
