package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/appkins-org/helm-api/internal/config"
)

type HandlerMapping map[string]Handler

// Api represents the HTTP API server with all its dependencies.
type Api struct {
	config     *config.Config
	logger     *slog.Logger
	httpServer *http.Server
	handlers   HandlerMapping
}

// New creates a new Api instance with the given configuration.
func New(cfg *config.Config, logger *slog.Logger) *Api {
	return &Api{
		config:   cfg,
		logger:   logger,
		handlers: make(HandlerMapping),
	}
}

func (a *Api) AddHandler(path string, handler Handler) {
	if handler != nil {
		a.handlers[path] = handler
	} else {
		a.logger.Warn("Attempted to add nil handler", "path", path)
	}
}

// Start initializes all dependencies and starts the HTTP server.
func (a *Api) Start() error {
	// Setup Helm directories first - panic if this fails as it's essential
	if err := a.config.SetupHelmDirectories(); err != nil {
		a.logger.Error("Failed to setup Helm directories", "error", err)
		panic("Failed to setup Helm directories: " + err.Error())
	}

	a.logger.Info("Helm directories setup completed",
		"plugins_dir", a.config.Helm.PluginsDirectory,
		"repo_cache", a.config.Helm.RepositoryCache,
		"repo_config", a.config.Helm.RepositoryConfig,
		"registry_config", a.config.Helm.RegistryConfig,
	)

	// Setup HTTP routes
	mux := http.NewServeMux()

	for path, handler := range a.handlers {
		mux.HandleFunc(path, handler.ServeHTTP)
	}

	// Setup middleware with logging and observability
	httpHandler := a.config.NewHTTPHandler(a.logger, mux)

	// Create and configure HTTP server
	a.httpServer = &http.Server{
		Addr:    a.config.GetAddress(),
		Handler: httpHandler,
		// Add reasonable timeouts
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	a.logger.Info("Starting HTTP server", "address", a.httpServer.Addr)

	// Start server - this blocks
	err := a.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		a.logger.Error("HTTP server failed to start", "error", err)
		return err
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server.
func (a *Api) Shutdown() error {
	if a.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		a.logger.Info("Shutting down HTTP server...")
		err := a.httpServer.Shutdown(ctx)
		if err != nil {
			a.logger.Error("Failed to shutdown HTTP server gracefully", "error", err)
			return err
		}

		a.httpServer = nil
		a.logger.Info("HTTP server shutdown complete")
	}

	return nil
}
