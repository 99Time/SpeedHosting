package dashboard

import (
	"context"

	"speedhosting/backend/internal/models"
	planservice "speedhosting/backend/internal/services/plan"
	serverservice "speedhosting/backend/internal/services/server"
)

type Service struct {
	plans   *planservice.Service
	servers *serverservice.Service
}

func NewService(plans *planservice.Service, servers *serverservice.Service) *Service {
	return &Service{plans: plans, servers: servers}
}

func (s *Service) GetOverview(ctx context.Context, user models.AuthenticatedUser) (models.DashboardOverview, error) {
	plan, err := s.plans.GetForUser(ctx, user.ID)
	if err != nil {
		return models.DashboardOverview{}, err
	}

	servers, err := s.servers.ListByOwner(ctx, user.ID)
	if err != nil {
		return models.DashboardOverview{}, err
	}

	summary := models.DashboardSummary{
		ServerCount: len(servers),
		MaxServers:  plan.MaxServers,
		MaxTickRate: plan.MaxTickRate,
	}

	for _, server := range servers {
		summary.TotalPlayers += server.PlayerCount
		if server.Status == "running" {
			summary.ActiveServers++
		}
	}

	return models.DashboardOverview{
		User:    user,
		Plan:    plan,
		Summary: summary,
		Servers: servers,
	}, nil
}
