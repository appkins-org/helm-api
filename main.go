package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/appkins-org/helm-api/api"
	"github.com/appkins-org/helm-api/api/health"
	"github.com/appkins-org/helm-api/api/root"
	"github.com/appkins-org/helm-api/api/template"
	"github.com/appkins-org/helm-api/internal/config"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Setup structured logging
	logger, err := cfg.SetupLogger()
	if err != nil {
		slog.Error("Failed to setup logger", "error", err)
		os.Exit(1)
	}

	// Log startup
	logger.Info("Starting Helm API server",
		"version", "1.0.0",
		"port", cfg.Server.Port,
		"log_level", cfg.Logging.Level,
		"log_format", cfg.Logging.Format,
	)

	// Create API instance
	apiServer := api.New(cfg, logger)

	apiServer.AddHandler("/", root.New(logger))
	apiServer.AddHandler("/health", health.New(logger))
	apiServer.AddHandler("/template", template.New(logger, cfg))

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in a goroutine
	go func() {
		if err := apiServer.Start(); err != nil {
			logger.Error("API server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	// Graceful shutdown
	logger.Info("Shutting down...")
	if err := apiServer.Shutdown(); err != nil {
		logger.Error("Failed to shutdown gracefully", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped gracefully")
}
