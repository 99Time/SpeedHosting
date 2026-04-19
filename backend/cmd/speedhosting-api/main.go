package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"speedhosting/backend/internal/config"
	"speedhosting/backend/internal/httpserver"
	"speedhosting/backend/internal/store"
)

func main() {
	cfg := config.Load()
	logger := log.New(os.Stdout, "[speedhosting] ", log.LstdFlags|log.Lshortfile)
	workingDirectory, _ := os.Getwd()
	logger.Printf("SpeedHosting effective database path: %s (cwd=%s)", cfg.DatabasePath, workingDirectory)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := store.Initialize(ctx, cfg)
	if err != nil {
		logger.Fatalf("database init failed: %v", err)
	}
	defer database.Close()

	router := httpserver.NewRouter(cfg, database, logger)

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Printf("SpeedHosting API listening on %s", cfg.HTTPAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
	}

	logger.Println("SpeedHosting API stopped")
}
