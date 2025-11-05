package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger interface
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Sync() error
}

// LoggerOption configures a Logger
type LoggerOption func(*loggerConfig) error

// loggerConfig holds logger configuration
type loggerConfig struct {
	format    string
	auditPath string
}

// WithFormat sets the log format (text or json)
func WithFormat(format string) LoggerOption {
	return func(c *loggerConfig) error {
		format = strings.ToLower(format)
		if format != "text" && format != "json" {
			return fmt.Errorf("invalid format: %s (must be 'text' or 'json')", format)
		}
		c.format = format
		return nil
	}
}

// WithAudit enables audit logging to a separate file
func WithAudit(auditPath string) LoggerOption {
	return func(c *loggerConfig) error {
		if auditPath == "" {
			return fmt.Errorf("audit path cannot be empty")
		}
		c.auditPath = auditPath
		return nil
	}
}

// New creates a new logger with optional configuration
func New(levelStr, outputPath string, opts ...LoggerOption) (Logger, error) {
	// Apply default config
	config := &loggerConfig{
		format: "text",
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	// Parse level
	level := parseSlogLevel(levelStr)

	// Open main output
	mainOutput, mainWriter, err := openOutput(outputPath)
	if err != nil {
		return nil, err
	}

	// Create main handler
	var mainHandler slog.Handler
	if config.format == "json" {
		mainHandler = slog.NewJSONHandler(mainWriter, &slog.HandlerOptions{Level: level})
	} else {
		mainHandler = slog.NewTextHandler(mainWriter, &slog.HandlerOptions{Level: level})
	}

	// If audit logging is enabled, combine handlers
	if config.auditPath != "" {
		auditFile, err := os.OpenFile(config.auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) // #nosec G304 G302 - configurable audit log path
		if err != nil {
			if mainOutput != os.Stdout && mainOutput != os.Stderr {
				_ = mainOutput.Close()
			}
			return nil, fmt.Errorf("failed to open audit log: %w", err)
		}

		// Audit logs are always JSON at INFO level
		auditHandler := slog.NewJSONHandler(auditFile, &slog.HandlerOptions{Level: slog.LevelInfo})
		mainHandler = NewMultiHandler(mainHandler, auditHandler)
	}

	return &slogLogger{
		logger: slog.New(mainHandler),
		output: mainOutput,
	}, nil
}

// openOutput opens the output writer and returns both a closer and writer
func openOutput(outputPath string) (io.WriteCloser, io.Writer, error) {
	switch strings.ToLower(outputPath) {
	case "stdout":
		return os.Stdout, os.Stdout, nil
	case "stderr":
		return os.Stderr, os.Stderr, nil
	default:
		file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) // #nosec G304 G302 - configurable log file path
		if err != nil {
			return nil, nil, err
		}
		return file, file, nil
	}
}

// parseSlogLevel converts a string level to slog.Level
func parseSlogLevel(levelStr string) slog.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "error":
		return slog.LevelError
	case "info":
		fallthrough
	default:
		return slog.LevelInfo
	}
}

// formatFields formats key-value pairs for logging (used by tests)
func formatFields(keysAndValues ...interface{}) string {
	var parts []string

	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			key := fmt.Sprintf("%v", keysAndValues[i])
			value := fmt.Sprintf("%v", keysAndValues[i+1])
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(parts, " ")
}

// slogLogger implements the Logger interface using Go's standard log/slog package
type slogLogger struct {
	logger *slog.Logger
	output io.WriteCloser
}

// Debug logs a debug message
func (l *slogLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debug(msg, keysAndValues...)
}

// Info logs an info message
func (l *slogLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, keysAndValues...)
}

// Error logs an error message
func (l *slogLogger) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Error(msg, keysAndValues...)
}

// Sync flushes any buffered log entries
func (l *slogLogger) Sync() error {
	if l.output != os.Stdout && l.output != os.Stderr {
		return l.output.Close()
	}
	return nil
}
