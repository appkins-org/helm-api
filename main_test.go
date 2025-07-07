package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	hhandler "github.com/appkins-org/helm-api/api/health"
	thandler "github.com/appkins-org/helm-api/api/template"
	"github.com/appkins-org/helm-api/internal/config"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
)

// TestMockHTTPServer creates a full mock HTTP server for integration testing.
func TestMockHTTPServer(t *testing.T) {
	// Create a new HTTP server with our routes
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/health",
		hhandler.New(
			logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))),
		).ServeHTTP,
	)
	mux.HandleFunc(
		"/template",
		thandler.New(
			logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))),
			config.DefaultConfig(),
		).ServeHTTP,
	)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test health endpoint
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health endpoint returned status %d, expected %d", resp.StatusCode, http.StatusOK)
	}
	// Test template endpoint with invalid request
	resp, err = http.Get(server.URL + "/template")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf(
			"Template endpoint returned status %d, expected %d",
			resp.StatusCode,
			http.StatusBadRequest,
		)
	}
}
