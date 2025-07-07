package config

import (
	"io"
	"log/slog"
	"os"

	"github.com/go-logr/logr"
)

// SetupLogger creates and configures a structured logger based on the configuration
func (c *Config) SetupLogger() (*slog.Logger, error) {
	// Determine output writer
	var writer io.Writer
	switch c.Logging.Output {
	case "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	default:
		// Assume it's a file path
		file, err := os.OpenFile(c.Logging.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		writer = file
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level:     c.GetLogLevel(),
		AddSource: c.Logging.Level == "debug",
	}

	// Create handler based on format
	var handler slog.Handler
	switch c.Logging.Format {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	default: // text
		handler = slog.NewTextHandler(writer, opts)
	}

	// Create logger
	logger := slog.New(handler)

	// Set as default logger
	slog.SetDefault(logger)

	return logger, nil
}

// SetupLogr creates a logr.Logger from the slog logger for backward compatibility
func (c *Config) SetupLogr(slogLogger *slog.Logger) logr.Logger {
	return logr.FromSlogHandler(slogLogger.Handler())
}

// LoggerWithContext adds context fields to the logger
func LoggerWithContext(logger *slog.Logger, keyvals ...any) *slog.Logger {
	return logger.With(keyvals...)
}
