package plan

import (
	"context"
	"database/sql"
	"fmt"

	"speedhosting/backend/internal/models"
	"speedhosting/backend/internal/planrules"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) GetForUser(ctx context.Context, userID int64) (models.Plan, error) {
	const query = `
		SELECT p.id, p.code, p.name, p.max_servers, p.max_tick_rate, p.max_admins,
		       p.allow_custom_mods, p.allow_advanced_config
		FROM users u
		JOIN plans p ON p.id = u.plan_id
		WHERE u.id = ?`

	var plan models.Plan
	var allowCustomMods int
	var allowAdvancedConfig int

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&plan.ID,
		&plan.Code,
		&plan.Name,
		&plan.MaxServers,
		&plan.MaxTickRate,
		&plan.MaxAdmins,
		&allowCustomMods,
		&allowAdvancedConfig,
	)
	if err != nil {
		return models.Plan{}, fmt.Errorf("load plan: %w", err)
	}

	plan.AllowCustomMods = allowCustomMods == 1
	plan.AllowAdvancedConfig = allowAdvancedConfig == 1
	plan = planrules.Apply(plan)

	return plan, nil
}
