package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test defaults
	if cfg.Server.Port != "8080" {
		t.Errorf("Expected default port 8080, got %s", cfg.Server.Port)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default log level info, got %s", cfg.Logging.Level)
	}

	if cfg.Logging.Format != "text" {
		t.Errorf("Expected default log format text, got %s", cfg.Logging.Format)
	}

	if cfg.Logging.Output != "stdout" {
		t.Errorf("Expected default log output stdout, got %s", cfg.Logging.Output)
	}

	if !cfg.Logging.HTTP.Enabled {
		t.Error("Expected HTTP logging to be enabled by default")
	}

	if cfg.Observability.Tracing.Enabled {
		t.Error("Expected tracing to be disabled by default")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Set environment variables
	originalEnvs := map[string]string{
		"PORT":                     os.Getenv("PORT"),
		"LOG_LEVEL":                os.Getenv("LOG_LEVEL"),
		"LOG_FORMAT":               os.Getenv("LOG_FORMAT"),
		"TRACING_ENABLED":          os.Getenv("TRACING_ENABLED"),
		"TRACING_WITH_SPAN_ID":     os.Getenv("TRACING_WITH_SPAN_ID"),
		"TRACING_WITH_TRACE_ID":    os.Getenv("TRACING_WITH_TRACE_ID"),
		"LOG_HTTP_ENABLED":         os.Getenv("LOG_HTTP_ENABLED"),
		"LOG_HTTP_REQUEST_BODY":    os.Getenv("LOG_HTTP_REQUEST_BODY"),
		"LOG_HTTP_RESPONSE_BODY":   os.Getenv("LOG_HTTP_RESPONSE_BODY"),
		"LOG_HTTP_REQUEST_HEADERS": os.Getenv("LOG_HTTP_REQUEST_HEADERS"),
	}

	// Clean up environment variables after test
	defer func() {
		for key, value := range originalEnvs {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Set test environment variables
	os.Setenv("PORT", "9090")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "json")
	os.Setenv("TRACING_ENABLED", "true")
	os.Setenv("TRACING_WITH_SPAN_ID", "true")
	os.Setenv("TRACING_WITH_TRACE_ID", "true")
	os.Setenv("LOG_HTTP_ENABLED", "false")
	os.Setenv("LOG_HTTP_REQUEST_BODY", "true")
	os.Setenv("LOG_HTTP_RESPONSE_BODY", "true")
	os.Setenv("LOG_HTTP_REQUEST_HEADERS", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test that environment variables were loaded
	if cfg.Server.Port != "9090" {
		t.Errorf("Expected port 9090, got %s", cfg.Server.Port)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level debug, got %s", cfg.Logging.Level)
	}

	if cfg.Logging.Format != "json" {
		t.Errorf("Expected log format json, got %s", cfg.Logging.Format)
	}

	if !cfg.Observability.Tracing.Enabled {
		t.Error("Expected tracing to be enabled")
	}

	if !cfg.Observability.Tracing.WithSpanID {
		t.Error("Expected WithSpanID to be enabled")
	}

	if !cfg.Observability.Tracing.WithTraceID {
		t.Error("Expected WithTraceID to be enabled")
	}

	if cfg.Logging.HTTP.Enabled {
		t.Error("Expected HTTP logging to be disabled")
	}

	if !cfg.Logging.HTTP.RequestBody {
		t.Error("Expected request body logging to be enabled")
	}

	if !cfg.Logging.HTTP.ResponseBody {
		t.Error("Expected response body logging to be enabled")
	}

	if !cfg.Logging.HTTP.RequestHeaders {
		t.Error("Expected request headers logging to be enabled")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		modifyFunc  func(*Config)
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			modifyFunc: func(cfg *Config) {
				// No modifications - should be valid
			},
			expectError: false,
		},
		{
			name: "invalid log level",
			modifyFunc: func(cfg *Config) {
				cfg.Logging.Level = "invalid"
			},
			expectError: true,
			errorMsg:    "invalid log level",
		},
		{
			name: "invalid log format",
			modifyFunc: func(cfg *Config) {
				cfg.Logging.Format = "invalid"
			},
			expectError: true,
			errorMsg:    "invalid log format",
		},
		{
			name: "empty port",
			modifyFunc: func(cfg *Config) {
				cfg.Server.Port = ""
			},
			expectError: true,
			errorMsg:    "server port cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modifyFunc(cfg)

			err := cfg.Validate()

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			} else if tt.expectError && err != nil && tt.errorMsg != "" {
				if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		level    string
		expected string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"invalid", "INFO"}, // Should default to INFO
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			cfg.Logging.Level = tt.level
			level := cfg.GetLogLevel()
			if level.String() != tt.expected {
				t.Errorf("Expected log level %s, got %s", tt.expected, level.String())
			}
		})
	}
}

func TestGetAddress(t *testing.T) {
	cfg := DefaultConfig()

	// Test with empty host
	cfg.Server.Host = ""
	cfg.Server.Port = "8080"
	if cfg.GetAddress() != ":8080" {
		t.Errorf("Expected :8080, got %s", cfg.GetAddress())
	}

	// Test with specific host
	cfg.Server.Host = "localhost"
	cfg.Server.Port = "9090"
	if cfg.GetAddress() != "localhost:9090" {
		t.Errorf("Expected localhost:9090, got %s", cfg.GetAddress())
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
