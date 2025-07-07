package health

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

// New creates a new health handler.
func New(logger *slog.Logger) api.Handler {
	return &handler{
		logger: logger,
	}
}

// ServeHTTP processes health check requests.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling health check", "path", r.URL.Path, "method", r.Method)

	response := map[string]string{
		"status": "healthy",
	}
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode health response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
