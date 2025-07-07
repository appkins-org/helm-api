package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	// Server configuration
	Server ServerConfig `yaml:"server" json:"server"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`

	// Observability configuration
	Observability ObservabilityConfig `yaml:"observability" json:"observability"`

	// Helm configuration
	Helm HelmConfig `yaml:"helm" json:"helm"`
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	Port string `yaml:"port" json:"port"`
	Host string `yaml:"host" json:"host"`
}

// HelmConfig holds Helm-related configuration.
type HelmConfig struct {
	// PluginsDirectory sets the plugins directory path
	PluginsDirectory string `yaml:"plugins_directory" json:"plugins_directory"`

	// RepositoryCache sets the repository cache directory path
	RepositoryCache string `yaml:"repository_cache" json:"repository_cache"`

	// RepositoryConfig sets the repository configuration file path
	RepositoryConfig string `yaml:"repository_config" json:"repository_config"`

	// RegistryConfig sets the registry configuration file path
	RegistryConfig string `yaml:"registry_config" json:"registry_config"`
}

// LoggingConfig holds logging-related configuration.
type LoggingConfig struct {
	// Level sets the log level (debug, info, warn, error)
	Level string `yaml:"level" json:"level"`

	// Format sets the log format (text, json)
	Format string `yaml:"format" json:"format"`

	// Output sets where logs are written (stdout, stderr, file path)
	Output string `yaml:"output" json:"output"`

	// HTTP logging configuration
	HTTP HTTPLoggingConfig `yaml:"http" json:"http"`
}

// HTTPLoggingConfig holds HTTP-specific logging configuration.
type HTTPLoggingConfig struct {
	// Enabled controls whether HTTP logging is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`

	// RequestBody controls whether request bodies are logged
	RequestBody bool `yaml:"request_body" json:"request_body"`

	// ResponseBody controls whether response bodies are logged
	ResponseBody bool `yaml:"response_body" json:"response_body"`

	// RequestHeaders controls whether request headers are logged
	RequestHeaders bool `yaml:"request_headers" json:"request_headers"`

	// ResponseHeaders controls whether response headers are logged
	ResponseHeaders bool `yaml:"response_headers" json:"response_headers"`
}

// ObservabilityConfig holds observability-related configuration.
type ObservabilityConfig struct {
	// Tracing configuration
	Tracing TracingConfig `yaml:"tracing" json:"tracing"`
}

// TracingConfig holds tracing-related configuration.
type TracingConfig struct {
	// Enabled controls whether tracing is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`

	// WithSpanID controls whether span IDs are included in logs
	WithSpanID bool `yaml:"with_span_id" json:"with_span_id"`

	// WithTraceID controls whether trace IDs are included in logs
	WithTraceID bool `yaml:"with_trace_id" json:"with_trace_id"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: "8080",
			Host: "",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
			HTTP: HTTPLoggingConfig{
				Enabled:         true,
				RequestBody:     false,
				ResponseBody:    false,
				RequestHeaders:  false,
				ResponseHeaders: false,
			},
		},
		Observability: ObservabilityConfig{
			Tracing: TracingConfig{
				Enabled:     false,
				WithSpanID:  false,
				WithTraceID: false,
			},
		},
		Helm: HelmConfig{
			PluginsDirectory: "/tmp/.helm/plugins",
			RepositoryCache:  "/tmp/.helm/repository",
			RepositoryConfig: "/tmp/.helm/repositories.yaml",
			RegistryConfig:   "/tmp/.helm/registry/config.json",
		},
	}
}

// LoadConfig loads configuration from environment variables and optional YAML file.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	// Load from YAML file if specified
	if configFile := os.Getenv("CONFIG_FILE"); configFile != "" {
		if err := loadFromYAML(cfg, configFile); err != nil {
			return nil, fmt.Errorf("failed to load config from YAML file %s: %w", configFile, err)
		}
	}

	// Override with environment variables
	loadFromEnv(cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// loadFromYAML loads configuration from a YAML file.
func loadFromYAML(cfg *Config, filename string) error {
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist: %s", filename)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables.
func loadFromEnv(cfg *Config) {
	// Server configuration
	if port := os.Getenv("PORT"); port != "" {
		cfg.Server.Port = port
	}
	if host := os.Getenv("HOST"); host != "" {
		cfg.Server.Host = host
	}

	// Logging configuration
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Logging.Level = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		cfg.Logging.Format = format
	}
	if output := os.Getenv("LOG_OUTPUT"); output != "" {
		cfg.Logging.Output = output
	}

	// HTTP logging configuration
	if enabled := os.Getenv("LOG_HTTP_ENABLED"); enabled != "" {
		if b, err := strconv.ParseBool(enabled); err == nil {
			cfg.Logging.HTTP.Enabled = b
		}
	}
	if reqBody := os.Getenv("LOG_HTTP_REQUEST_BODY"); reqBody != "" {
		if b, err := strconv.ParseBool(reqBody); err == nil {
			cfg.Logging.HTTP.RequestBody = b
		}
	}
	if respBody := os.Getenv("LOG_HTTP_RESPONSE_BODY"); respBody != "" {
		if b, err := strconv.ParseBool(respBody); err == nil {
			cfg.Logging.HTTP.ResponseBody = b
		}
	}
	if reqHeaders := os.Getenv("LOG_HTTP_REQUEST_HEADERS"); reqHeaders != "" {
		if b, err := strconv.ParseBool(reqHeaders); err == nil {
			cfg.Logging.HTTP.RequestHeaders = b
		}
	}
	if respHeaders := os.Getenv("LOG_HTTP_RESPONSE_HEADERS"); respHeaders != "" {
		if b, err := strconv.ParseBool(respHeaders); err == nil {
			cfg.Logging.HTTP.ResponseHeaders = b
		}
	}

	// Observability configuration
	if enabled := os.Getenv("TRACING_ENABLED"); enabled != "" {
		if b, err := strconv.ParseBool(enabled); err == nil {
			cfg.Observability.Tracing.Enabled = b
		}
	}
	if spanID := os.Getenv("TRACING_WITH_SPAN_ID"); spanID != "" {
		if b, err := strconv.ParseBool(spanID); err == nil {
			cfg.Observability.Tracing.WithSpanID = b
		}
	}
	if traceID := os.Getenv("TRACING_WITH_TRACE_ID"); traceID != "" {
		if b, err := strconv.ParseBool(traceID); err == nil {
			cfg.Observability.Tracing.WithTraceID = b
		}
	}

	// Helm configuration
	if pluginsDir := os.Getenv("HELM_PLUGINS_DIRECTORY"); pluginsDir != "" {
		cfg.Helm.PluginsDirectory = pluginsDir
	}
	if repoCache := os.Getenv("HELM_REPOSITORY_CACHE"); repoCache != "" {
		cfg.Helm.RepositoryCache = repoCache
	}
	if repoConfig := os.Getenv("HELM_REPOSITORY_CONFIG"); repoConfig != "" {
		cfg.Helm.RepositoryConfig = repoConfig
	}
	if registryConfig := os.Getenv("HELM_REGISTRY_CONFIG"); registryConfig != "" {
		cfg.Helm.RegistryConfig = registryConfig
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Validate log level
	level := strings.ToLower(c.Logging.Level)
	switch level {
	case "debug", "info", "warn", "error":
		c.Logging.Level = level
	default:
		return fmt.Errorf(
			"invalid log level: %s (must be debug, info, warn, or error)",
			c.Logging.Level,
		)
	}

	// Validate log format
	format := strings.ToLower(c.Logging.Format)
	switch format {
	case "text", "json":
		c.Logging.Format = format
	default:
		return fmt.Errorf("invalid log format: %s (must be text or json)", c.Logging.Format)
	}

	// Validate log output
	output := strings.ToLower(c.Logging.Output)
	switch output {
	case "stdout", "stderr":
		c.Logging.Output = output
	default:
		// Check if it's a file path
		if c.Logging.Output != "" {
			dir := filepath.Dir(c.Logging.Output)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return fmt.Errorf("log output directory does not exist: %s", dir)
			}
		}
	}

	// Validate port
	if c.Server.Port == "" {
		return fmt.Errorf("server port cannot be empty")
	}

	return nil
}

// GetLogLevel returns the slog.Level for the configured log level.
func (c *Config) GetLogLevel() slog.Level {
	switch strings.ToLower(c.Logging.Level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// GetAddress returns the full server address.
func (c *Config) GetAddress() string {
	if c.Server.Host == "" {
		return ":" + c.Server.Port
	}
	return c.Server.Host + ":" + c.Server.Port
}

// SetupHelmDirectories creates all required Helm directories and configuration files.
func (c *Config) SetupHelmDirectories() error {
	// Create plugins directory
	if err := os.MkdirAll(c.Helm.PluginsDirectory, 0o755); err != nil {
		return fmt.Errorf("failed to create plugins directory %s: %w", c.Helm.PluginsDirectory, err)
	}

	// Create repository cache directory
	if err := os.MkdirAll(c.Helm.RepositoryCache, 0o755); err != nil {
		return fmt.Errorf(
			"failed to create repository cache directory %s: %w",
			c.Helm.RepositoryCache,
			err,
		)
	}

	// Create directory for repository config file
	repoConfigDir := filepath.Dir(c.Helm.RepositoryConfig)
	if err := os.MkdirAll(repoConfigDir, 0o755); err != nil {
		return fmt.Errorf("failed to create repository config directory %s: %w", repoConfigDir, err)
	}

	// Create directory for registry config file
	registryConfigDir := filepath.Dir(c.Helm.RegistryConfig)
	if err := os.MkdirAll(registryConfigDir, 0o755); err != nil {
		return fmt.Errorf(
			"failed to create registry config directory %s: %w",
			registryConfigDir,
			err,
		)
	}

	// Create empty registry config file if it doesn't exist
	if _, err := os.Stat(c.Helm.RegistryConfig); os.IsNotExist(err) {
		emptyConfig := `{"auths":{}}`
		if err := os.WriteFile(c.Helm.RegistryConfig, []byte(emptyConfig), 0o644); err != nil {
			return fmt.Errorf(
				"failed to create registry config file %s: %w",
				c.Helm.RegistryConfig,
				err,
			)
		}
	}

	return nil
}
