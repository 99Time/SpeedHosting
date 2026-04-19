package updates

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"speedhosting/backend/internal/models"
)

type Service struct {
	path   string
	logger *log.Logger
}

func NewService(path string, logger *log.Logger) *Service {
	return &Service{
		path:   strings.TrimSpace(path),
		logger: logger,
	}
}

func (s *Service) List() ([]models.UpdateEntry, error) {
	if s.path == "" {
		return nil, fmt.Errorf("updates path is not configured")
	}

	content, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read updates file: %w", err)
	}

	entries := make([]models.UpdateEntry, 0)
	if err := json.Unmarshal(content, &entries); err != nil {
		return nil, fmt.Errorf("parse updates file: %w", err)
	}

	normalized := make([]models.UpdateEntry, 0, len(entries))
	for index, entry := range entries {
		entry.Title = strings.TrimSpace(entry.Title)
		entry.ShortDescription = strings.TrimSpace(entry.ShortDescription)
		entry.Content = strings.TrimSpace(entry.Content)
		entry.Tag = strings.TrimSpace(entry.Tag)
		entry.CreatedAt = strings.TrimSpace(entry.CreatedAt)
		entry.Icon = strings.TrimSpace(entry.Icon)
		if entry.ID == "" {
			entry.ID = fmt.Sprintf("update-%d", index+1)
		}
		if entry.Title == "" || entry.ShortDescription == "" || entry.Content == "" || entry.Tag == "" || entry.CreatedAt == "" {
			continue
		}
		normalized = append(normalized, entry)
	}

	sort.SliceStable(normalized, func(left, right int) bool {
		leftTime := parseUpdateDate(normalized[left].CreatedAt)
		rightTime := parseUpdateDate(normalized[right].CreatedAt)
		if leftTime.Equal(rightTime) {
			return normalized[left].Title > normalized[right].Title
		}
		return leftTime.After(rightTime)
	})

	s.logf("[updates] loaded count=%d path=%s", len(normalized), s.path)

	return normalized, nil
}

func parseUpdateDate(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC()
		}
	}

	return time.Time{}
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}
