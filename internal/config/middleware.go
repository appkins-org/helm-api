package config

import (
	"log/slog"
	"net/http"

	sloghttp "github.com/samber/slog-http"
)

// SetupHTTPMiddleware configures HTTP middleware with logging and observability.
func (c *Config) SetupHTTPMiddleware(logger *slog.Logger, handler http.Handler) http.Handler {
	if !c.Logging.HTTP.Enabled {
		return handler
	}

	// Configure slog-http
	config := sloghttp.Config{
		WithSpanID:  c.Observability.Tracing.WithSpanID,
		WithTraceID: c.Observability.Tracing.WithTraceID,

		// Configure what to log
		WithRequestBody:    c.Logging.HTTP.RequestBody,
		WithResponseBody:   c.Logging.HTTP.ResponseBody,
		WithRequestHeader:  c.Logging.HTTP.RequestHeaders,
		WithResponseHeader: c.Logging.HTTP.ResponseHeaders,

		// Filter sensitive data
		Filters: []sloghttp.Filter{
			sloghttp.IgnorePathContains("/health"),
		},
	}

	// Apply recovery middleware first
	handler = sloghttp.Recovery(handler)

	// Apply logging middleware
	handler = sloghttp.NewWithConfig(logger, config)(handler)

	return handler
}

// NewHTTPHandler creates a new HTTP handler with logging middleware.
func (c *Config) NewHTTPHandler(logger *slog.Logger, mux *http.ServeMux) http.Handler {
	return c.SetupHTTPMiddleware(logger, mux)
}
