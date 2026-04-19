package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"speedhosting/backend/internal/models"
)

type Service struct {
	db *sql.DB
}

type TrackEventInput struct {
	Name            string
	Source          string
	Route           string
	LandingPath     string
	FullURL         string
	SessionID       string
	ClientTimestamp string
	ServerID        *int64
	Metadata        map[string]any
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) TrackEvent(ctx context.Context, userID *int64, input TrackEventInput) error {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil
	}

	route := strings.TrimSpace(input.Route)
	if route == "" {
		route = "/"
	}

	source := sanitizeSource(input.Source)
	metadataJSON, err := marshalMetadata(input.Metadata)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO analytics_events (
			event_name, source, route, landing_path, full_url, session_id, user_id, server_id, client_timestamp, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		name,
		source,
		route,
		nullableString(input.LandingPath),
		nullableString(input.FullURL),
		nullableString(input.SessionID),
		nullableInt64(userID),
		nullableInt64(input.ServerID),
		nullableString(input.ClientTimestamp),
		metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("insert analytics event: %w", err)
	}

	return nil
}

func (s *Service) LoadAttributionSummary(ctx context.Context) ([]models.AttributionSourceReport, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			source,
			SUM(CASE WHEN event_name = 'puck_landing_view' THEN 1 ELSE 0 END) AS landing_views,
			SUM(CASE WHEN event_name = 'puck_cta_click' THEN 1 ELSE 0 END) AS cta_clicks,
			SUM(CASE WHEN event_name = 'register_view' THEN 1 ELSE 0 END) AS register_views,
			SUM(CASE WHEN event_name = 'register_submit' THEN 1 ELSE 0 END) AS register_submits,
			SUM(CASE WHEN event_name = 'register_success' THEN 1 ELSE 0 END) AS register_successes,
			SUM(CASE WHEN event_name = 'first_server_create_click' THEN 1 ELSE 0 END) AS first_server_clicks,
			SUM(CASE WHEN event_name = 'first_server_created' THEN 1 ELSE 0 END) AS first_server_created,
			SUM(CASE WHEN event_name = 'pro_upgrade_view' THEN 1 ELSE 0 END) AS pro_upgrade_views,
			SUM(CASE WHEN event_name = 'pro_upgrade_click' THEN 1 ELSE 0 END) AS pro_upgrade_clicks,
			SUM(CASE WHEN event_name = 'pro_upgrade_success' THEN 1 ELSE 0 END) AS pro_upgrade_successes
		FROM analytics_events
		GROUP BY source
		ORDER BY landing_views DESC, register_successes DESC, source ASC`)
	if err != nil {
		return nil, fmt.Errorf("load attribution summary: %w", err)
	}
	defer rows.Close()

	reports := make([]models.AttributionSourceReport, 0)
	for rows.Next() {
		var report models.AttributionSourceReport
		if err := rows.Scan(
			&report.Source,
			&report.LandingViews,
			&report.CTAClicks,
			&report.RegisterViews,
			&report.RegisterSubmits,
			&report.RegisterSuccesses,
			&report.FirstServerClicks,
			&report.FirstServerCreated,
			&report.ProUpgradeViews,
			&report.ProUpgradeClicks,
			&report.ProUpgradeSuccesses,
		); err != nil {
			return nil, fmt.Errorf("scan attribution summary: %w", err)
		}

		report.RegisterConversionPct = percent(report.RegisterSuccesses, report.LandingViews)
		report.ServerConversionPct = percent(report.FirstServerCreated, report.LandingViews)
		report.ProConversionPct = percent(report.ProUpgradeSuccesses, report.LandingViews)
		reports = append(reports, report)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attribution summary: %w", err)
	}

	return reports, nil
}

func (s *Service) TrackProUpgradeSuccess(ctx context.Context, userID int64) error {
	source, route, sessionID, fullURL, landingPath, clientTimestamp, err := s.latestUpgradeAttribution(ctx, userID)
	if err != nil {
		return err
	}

	return s.TrackEvent(ctx, &userID, TrackEventInput{
		Name:            "pro_upgrade_success",
		Source:          source,
		Route:           route,
		LandingPath:     landingPath,
		FullURL:         fullURL,
		SessionID:       sessionID,
		ClientTimestamp: clientTimestamp,
	})
}

func (s *Service) latestUpgradeAttribution(ctx context.Context, userID int64) (string, string, string, string, string, string, error) {
	var source sql.NullString
	var route sql.NullString
	var sessionID sql.NullString
	var fullURL sql.NullString
	var landingPath sql.NullString
	var clientTimestamp sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT source, route, session_id, full_url, landing_path, client_timestamp
		FROM analytics_events
		WHERE user_id = ? AND event_name = 'pro_upgrade_click'
		ORDER BY COALESCE(client_timestamp, created_at) DESC, id DESC
		LIMIT 1`, userID).Scan(&source, &route, &sessionID, &fullURL, &landingPath, &clientTimestamp)
	if err == nil {
		return source.String, route.String, sessionID.String, fullURL.String, landingPath.String, clientTimestamp.String, nil
	}

	if err != nil && err != sql.ErrNoRows {
		return "", "", "", "", "", "", fmt.Errorf("load latest upgrade attribution: %w", err)
	}

	var latestSource sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT latest_acquisition_source FROM users WHERE id = ?`, userID).Scan(&latestSource); err != nil {
		if err == sql.ErrNoRows {
			return "direct", "/app/account", "", "", "", "", nil
		}

		return "", "", "", "", "", "", fmt.Errorf("load latest acquisition source: %w", err)
	}

	return sanitizeSource(latestSource.String), "/app/account", "", "", "", "", nil
}

func sanitizeSource(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "direct"
	}

	buffer := strings.Builder{}
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
			buffer.WriteRune(character)
		case character >= '0' && character <= '9':
			buffer.WriteRune(character)
		case character == '-' || character == '_':
			buffer.WriteRune(character)
		}
	}

	if buffer.Len() == 0 {
		return "direct"
	}

	return buffer.String()
}

func marshalMetadata(metadata map[string]any) (any, error) {
	if len(metadata) == 0 {
		return nil, nil
	}

	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal analytics metadata: %w", err)
	}

	return string(encoded), nil
}

func percent(numerator int, denominator int) float64 {
	if denominator == 0 {
		return 0
	}

	return float64(numerator) * 100 / float64(denominator)
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}

	return *value
}
