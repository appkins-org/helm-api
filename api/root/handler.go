package root

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/appkins-org/helm-api/api"
)

// handler handles health check requests.
type handler struct {
	logger *slog.Logger
}

// NewHandler creates a new health handler.
func New(logger *slog.Logger) api.Handler {
	return &handler{
		logger: logger,
	}
}

// ServeHTTP handles the root endpoint.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling root request", "path", r.URL.Path, "method", r.Method)

	response := map[string]string{
		"message": "Helm API Server",
		"version": "1.0.0",
	}
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode root response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
