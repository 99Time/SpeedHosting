package httpserver

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"speedhosting/backend/internal/config"
	"speedhosting/backend/internal/httpserver/handlers"
	appmiddleware "speedhosting/backend/internal/httpserver/middleware"
	adminservice "speedhosting/backend/internal/services/admin"
	analyticsservice "speedhosting/backend/internal/services/analytics"
	authservice "speedhosting/backend/internal/services/auth"
	dashboardservice "speedhosting/backend/internal/services/dashboard"
	planservice "speedhosting/backend/internal/services/plan"
	puckservice "speedhosting/backend/internal/services/puck"
	rankedservice "speedhosting/backend/internal/services/ranked"
	serverservice "speedhosting/backend/internal/services/server"
	updatesservice "speedhosting/backend/internal/services/updates"
)

func NewRouter(cfg config.Config, database *sql.DB, logger *log.Logger) http.Handler {
	auth := authservice.NewService(database, cfg.SessionTTL)
	analytics := analyticsservice.NewService(database)
	plans := planservice.NewService(database)
	ranked := rankedservice.NewService(database, logger, cfg.DatabasePath, cfg.RankedDataPath, cfg.RankedMMRPath, cfg.RankedStarsPath, cfg.RankedLinkCodeTTL)
	puck := puckservice.NewService(database, logger, ranked)
	servers := serverservice.NewService(database, plans, analytics, logger, cfg)
	updates := updatesservice.NewService(cfg.UpdatesPath, logger)
	dashboard := dashboardservice.NewService(plans, servers)
	admin := adminservice.NewService(database, analytics)

	healthHandler := handlers.HealthHandler{}
	authHandler := handlers.NewAuthHandler(auth, analytics, cfg)
	analyticsHandler := handlers.NewAnalyticsHandler(analytics, auth, cfg.SessionCookieName)
	planHandler := handlers.NewPlanHandler(plans, servers)
	rankedHandler := handlers.NewRankedHandler(ranked)
	puckHandler := handlers.NewPuckHandler(puck)
	updatesHandler := handlers.NewUpdatesHandler(updates)
	dashboardHandler := handlers.NewDashboardHandler(dashboard)
	serverHandler := handlers.NewServerHandler(servers)
	adminHandler := handlers.NewAdminHandler(admin, servers)
	matchResultsHandler := handlers.NewMatchResultsHandler(puck)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(chimiddleware.RealIP)
	router.Use(chimiddleware.Recoverer)
	router.Use(chimiddleware.Timeout(30 * time.Second))
	router.Use(chimiddleware.RequestLogger(&chimiddleware.DefaultLogFormatter{Logger: logger}))
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Get("/api/v1/health", healthHandler.ServeHTTP)
	router.Get("/api/updates", updatesHandler.List)
	router.Get("/api/ranked/leaderboard", rankedHandler.Leaderboard)
	router.Get("/api/ranked/rank", rankedHandler.Rank)
	router.Route("/api/ranked/link", func(linkRoutes chi.Router) {
		linkRoutes.Use(appmiddleware.RequireBearerAPIKey(cfg.RankedLinkAPIKey))
		linkRoutes.Post("/", rankedHandler.LinkSteam)
		linkRoutes.Post("/request", rankedHandler.RequestLink)
		linkRoutes.Get("/status", rankedHandler.LinkStatus)
		linkRoutes.Post("/complete", rankedHandler.CompleteLink)
	})
	router.Route("/api/ranked/results", func(resultRoutes chi.Router) {
		resultRoutes.Use(appmiddleware.RequireBearerAPIKey(cfg.RankedLinkAPIKey))
		resultRoutes.Get("/recent", matchResultsHandler.Recent)
	})
	router.Route("/api/ranked/matches", func(matchRoutes chi.Router) {
		matchRoutes.Use(appmiddleware.RequireBearerAPIKey(cfg.RankedLinkAPIKey))
		matchRoutes.Get("/latest", matchResultsHandler.Latest)
	})
	router.Route("/api/puck", func(puckRoutes chi.Router) {
		puckRoutes.Use(appmiddleware.RequirePuckAPIKey(cfg.PuckAPIKey))
		puckRoutes.Get("/players/{steamID}", puckHandler.GetPlayerState)
		puckRoutes.Post("/matches", puckHandler.ReportMatch)
		puckRoutes.Post("/moderation/mute", puckHandler.Mute)
		puckRoutes.Post("/moderation/tempban", puckHandler.TempBan)
	})

	router.Route("/api/v1", func(api chi.Router) {
		api.Post("/analytics/events", analyticsHandler.Track)
		api.Post("/auth/register", authHandler.Register)
		api.Post("/auth/login", authHandler.Login)
		api.Post("/auth/logout", authHandler.Logout)

		api.Group(func(protected chi.Router) {
			protected.Use(appmiddleware.RequireAuth(auth, cfg.SessionCookieName))
			protected.Get("/auth/me", authHandler.Me)
			protected.Get("/dashboard", dashboardHandler.Overview)
			protected.Get("/account/plan", planHandler.Show)
			protected.Get("/servers", serverHandler.List)
			protected.Post("/servers", serverHandler.Create)
			protected.Get("/servers/{serverID}", serverHandler.Get)
			protected.Post("/servers/{serverID}/actions", serverHandler.Action)
			protected.Patch("/servers/{serverID}/config", serverHandler.UpdateConfig)
			protected.Delete("/servers/{serverID}", serverHandler.Delete)

			protected.Group(func(adminRoutes chi.Router) {
				adminRoutes.Use(appmiddleware.RequireAdmin)
				adminRoutes.Get("/admin/overview", adminHandler.Overview)
				adminRoutes.Patch("/admin/users/{userID}/plan", adminHandler.UpdateUserPlan)
				adminRoutes.Post("/admin/servers/{serverID}/actions", adminHandler.ServerAction)
				adminRoutes.Patch("/admin/servers/{serverID}/config", adminHandler.UpdateServerConfig)
				adminRoutes.Delete("/admin/servers/{serverID}", adminHandler.DeleteServer)
			})
		})
	})

	return router
}
