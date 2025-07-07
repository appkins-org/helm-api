package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/appkins-org/helm-api/internal/config"
	"github.com/appkins-org/helm-api/internal/template"
	"github.com/go-logr/logr"
	"github.com/gorilla/schema"
)

var (
	decoder    = schema.NewDecoder()
	logger     *slog.Logger
	logrLogger logr.Logger
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Setup structured logging
	logger, err = cfg.SetupLogger()
	if err != nil {
		slog.Error("Failed to setup logger", "error", err)
		os.Exit(1)
	}

	// Setup logr logger for backward compatibility
	logrLogger = cfg.SetupLogr(logger)

	// Log startup
	logger.Info("Starting Helm API server",
		"version", "1.0.0",
		"port", cfg.Server.Port,
		"log_level", cfg.Logging.Level,
		"log_format", cfg.Logging.Format,
	)

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/template", handleTemplate)

	// Setup middleware with logging and observability
	handler := cfg.NewHTTPHandler(logger, mux)

	// Create server
	server := &http.Server{
		Addr:    cfg.GetAddress(),
		Handler: handler,
	}

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in a goroutine
	go func() {
		logger.Info("Server listening", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	// Graceful shutdown
	logger.Info("Server shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped gracefully")
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	log := logger
	if log == nil {
		log = slog.Default()
	}
	log.Debug("Handling root request", "path", r.URL.Path, "method", r.Method)

	response := map[string]string{
		"message": "Helm API Server",
		"version": "1.0.0",
	}
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error("Failed to encode root response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	log := logger
	if log == nil {
		log = slog.Default()
	}
	log.Debug("Handling health check", "path", r.URL.Path, "method", r.Method)

	response := map[string]string{
		"status": "healthy",
	}
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error("Failed to encode health response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func handleTemplate(w http.ResponseWriter, r *http.Request) {
	log := logger
	if log == nil {
		log = slog.Default()
	}
	reqLogger := log.With("path", r.URL.Path, "method", r.Method)
	reqLogger.Debug("Handling template request")

	if r.Method != http.MethodGet {
		reqLogger.Warn("Method not allowed", "method", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters into TemplateRequest
	req, err := parseTemplateRequest(r)
	if err != nil {
		reqLogger.Error("Failed to parse template request", "error", err)
		w.Header().Set("Content-Type", "application/json")
		errorResponse := map[string]string{"error": err.Error()}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(errorResponse)
		return
	}

	reqLogger = reqLogger.With(
		"chart", req.Chart,
		"repository", req.Repository,
		"chart_version", req.ChartVersion,
		"release_name", req.ReleaseName,
		"namespace", req.Namespace,
	)
	reqLogger.Info("Processing template request")

	result, err := template.ProcessTemplate(req)
	if err != nil {
		// Check if it's a validation error (400) vs processing error (500)
		if strings.Contains(err.Error(), "chart is required") ||
			strings.Contains(err.Error(), "invalid") {
			reqLogger.Warn("Template request validation failed", "error", err)
			w.Header().Set("Content-Type", "application/json")
			errorResponse := map[string]string{"error": err.Error()}
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse)
			return
		}
		reqLogger.Error("Template processing failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		errorResponse := map[string]string{"error": err.Error()}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(errorResponse)
		return
	}

	reqLogger.Info("Template processed successfully",
		"result_length", len(result),
		"has_content", result != "",
	)

	w.Header().Set("Content-Type", "application/x-yaml")
	_, _ = w.Write([]byte(result))
}

// parseTemplateRequest parses HTTP query parameters into a TemplateRequest.
func parseTemplateRequest(r *http.Request) (template.TemplateRequest, error) {
	query := r.URL.Query()

	log := logger
	if log == nil {
		log = slog.Default()
	}
	log.Debug("Parsing template request",
		"query_params_count", len(query),
		"has_chart", query.Get("chart") != "",
		"has_repository", query.Get("repository") != "",
	)

	req := template.TemplateRequest{}

	// First try to decode with the schema decoder
	err := decoder.Decode(&req, query)
	if err != nil {
		log.Error("Failed to decode query parameters", "error", err)
		return req, err
	}

	log.Debug("Template request parsed successfully",
		"chart", req.Chart,
		"repository", req.Repository,
		"chart_version", req.ChartVersion,
	)

	return req, nil
}
