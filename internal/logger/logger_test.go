package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Stdout(t *testing.T) {
	log, err := New("info", "stdout")
	require.NoError(t, err)
	require.NotNil(t, log)
	// Logger created successfully - level checking via behavior is done in other tests
}

func TestNew_Stderr(t *testing.T) {
	log, err := New("debug", "stderr")
	require.NoError(t, err)
	require.NotNil(t, log)
	// Logger created successfully - level checking via behavior is done in other tests
}

func TestNew_File(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("error", logPath)
	require.NoError(t, err)
	require.NotNil(t, log)

	// Logger created successfully - level checking via behavior is done in other tests

	// Clean up
	_ = log.Sync()
}

func TestNew_InvalidPath(t *testing.T) {
	log, err := New("info", "/nonexistent/directory/test.log")
	assert.Error(t, err)
	assert.Nil(t, log)
}

func TestLogger_Info(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("info", logPath)
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Info("test message", "key1", "value1", "key2", 123)

	// Read log file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logContent := string(content)
	assert.Contains(t, logContent, "level=INFO")
	assert.Contains(t, logContent, "test message")
	assert.Contains(t, logContent, "key1=value1")
	assert.Contains(t, logContent, "key2=123")
}

func TestLogger_Error(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("info", logPath)
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Error("error message", "error", "something went wrong")

	// Read log file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logContent := string(content)
	assert.Contains(t, logContent, "level=ERROR")
	assert.Contains(t, logContent, "error message")
	assert.Contains(t, logContent, "error=\"something went wrong\"")
}

func TestLogger_Debug(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("debug", logPath)
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Debug("debug message", "debug", "info")

	// Read log file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logContent := string(content)
	assert.Contains(t, logContent, "level=DEBUG")
	assert.Contains(t, logContent, "debug message")
}

func TestLogger_Debug_NotLogged(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("info", logPath)
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Debug("debug message", "debug", "info")

	// Read log file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logContent := string(content)
	assert.NotContains(t, logContent, "level=DEBUG")
	assert.NotContains(t, logContent, "debug message")
}

func TestLogger_ErrorLevel_FiltersInfo(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("error", logPath)
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Info("info message")
	log.Error("error message")

	// Read log file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logContent := string(content)
	assert.NotContains(t, logContent, "level=INFO")
	assert.Contains(t, logContent, "level=ERROR")
}

func TestLogger_WithAudit(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	auditPath := filepath.Join(tmpDir, "audit.log")

	// Use functional options pattern
	log, err := New("info", logPath, WithAudit(auditPath))
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Info("audit test", "user", "testuser", "action", "encrypt")

	// Check audit log (slog JSON format)
	auditContent, err := os.ReadFile(auditPath)
	require.NoError(t, err)

	auditLog := string(auditContent)
	assert.Contains(t, auditLog, "audit test")
	assert.Contains(t, auditLog, "\"user\":\"testuser\"")
	assert.Contains(t, auditLog, "\"action\":\"encrypt\"")
}

func TestFormatFields(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected string
	}{
		{
			name:     "single pair",
			input:    []interface{}{"key", "value"},
			expected: "key=value",
		},
		{
			name:     "multiple pairs",
			input:    []interface{}{"key1", "value1", "key2", "value2"},
			expected: "key1=value1 key2=value2",
		},
		{
			name:     "integer value",
			input:    []interface{}{"count", 42},
			expected: "count=42",
		},
		{
			name:     "odd number of args (last key ignored)",
			input:    []interface{}{"key1", "value1", "key2"},
			expected: "key1=value1",
		},
		{
			name:     "empty",
			input:    []interface{}{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFields(tt.input...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogger_MessageWithoutFields(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("info", logPath)
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Info("simple message")

	// Read log file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logContent := string(content)
	assert.Contains(t, logContent, "level=INFO")
	assert.Contains(t, logContent, "simple message")
	// Should not have extra spaces for empty fields
	lines := strings.Split(strings.TrimSpace(logContent), "\n")
	assert.Equal(t, 1, len(lines))
	assert.NotContains(t, lines[0], "  ") // No double spaces
}

func TestLogger_WithFormat_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("info", logPath, WithFormat("json"))
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Info("json test", "key", "value")

	// Read log file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logContent := string(content)
	assert.Contains(t, logContent, `"level":"INFO"`)
	assert.Contains(t, logContent, `"msg":"json test"`)
	assert.Contains(t, logContent, `"key":"value"`)
}

func TestLogger_WithFormat_Text(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("info", logPath, WithFormat("text"))
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Info("text test", "key", "value")

	// Read log file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	logContent := string(content)
	assert.Contains(t, logContent, "level=INFO")
	assert.Contains(t, logContent, "text test")
	assert.Contains(t, logContent, "key=value")
}

func TestLogger_WithFormat_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("info", logPath, WithFormat("xml"))
	assert.Error(t, err)
	assert.Nil(t, log)
	assert.Contains(t, err.Error(), "invalid format")
}

func TestLogger_WithAudit_EmptyPath(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	log, err := New("info", logPath, WithAudit(""))
	assert.Error(t, err)
	assert.Nil(t, log)
	assert.Contains(t, err.Error(), "audit path cannot be empty")
}

func TestLogger_MultipleOptions(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	auditPath := filepath.Join(tmpDir, "audit.log")

	log, err := New("info", logPath, WithFormat("json"), WithAudit(auditPath))
	require.NoError(t, err)
	defer func() { _ = log.Sync() }()

	log.Info("multi option test", "key", "value")

	// Check main log is JSON
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"level":"INFO"`)

	// Check audit log exists
	auditContent, err := os.ReadFile(auditPath)
	require.NoError(t, err)
	assert.Contains(t, string(auditContent), "multi option test")
}
