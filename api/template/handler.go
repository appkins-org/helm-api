package template

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/appkins-org/helm-api/api"
	"github.com/appkins-org/helm-api/internal/config"
	"github.com/appkins-org/helm-api/internal/template"
	"github.com/gorilla/schema"
)

// handler handles Helm template requests.
type handler struct {
	logger  *slog.Logger
	config  *config.Config
	decoder *schema.Decoder
}

// New creates a new template handler.
func New(logger *slog.Logger, cfg *config.Config) api.Handler {
	return &handler{
		logger:  logger,
		config:  cfg,
		decoder: schema.NewDecoder(),
	}
}

// ServeHTTP processes template requests.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqLogger := h.logger.With("path", r.URL.Path, "method", r.Method)
	reqLogger.Debug("Handling template request")

	if r.Method != http.MethodGet {
		reqLogger.Warn("Method not allowed", "method", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters into TemplateRequest
	req, err := h.parseTemplateRequest(r)
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

	// Process the template with configuration
	result, err := template.ProcessTemplateWithConfig(req, h.config)
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
func (h *handler) parseTemplateRequest(r *http.Request) (template.TemplateRequest, error) {
	query := r.URL.Query()

	h.logger.Debug("Parsing template request",
		"query_params_count", len(query),
		"has_chart", query.Get("chart") != "",
		"has_repository", query.Get("repository") != "",
	)

	req := template.TemplateRequest{}

	// Decode with the schema decoder
	err := h.decoder.Decode(&req, query)
	if err != nil {
		h.logger.Error("Failed to decode query parameters", "error", err)
		return req, err
	}

	h.logger.Debug("Template request parsed successfully",
		"chart", req.Chart,
		"repository", req.Repository,
		"chart_version", req.ChartVersion,
	)

	return req, nil
}
