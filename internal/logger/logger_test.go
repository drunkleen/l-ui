package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestTextFormatOutput(t *testing.T) {
	os.Setenv("LUI_LOG_FORMAT", "text")
	defer os.Unsetenv("LUI_LOG_FORMAT")

	var buf bytes.Buffer
	InitLogger("info")
	slogger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Fatalf("expected 'test message' in output, got: %s", output)
	}
	if strings.Contains(output, "{") {
		t.Fatalf("expected text format, got JSON: %s", output)
	}
}

func TestJSONFormatOutput(t *testing.T) {
	os.Setenv("LUI_LOG_FORMAT", "json")
	defer os.Unsetenv("LUI_LOG_FORMAT")

	var buf bytes.Buffer
	InitLogger("info")
	slogger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	Info("json test message")

	output := buf.String()
	var logEntry map[string]any
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected JSON output, got: %s, error: %v", output, err)
	}
	if logEntry["msg"] != "json test message" {
		t.Fatalf("expected msg 'json test message', got: %v", logEntry["msg"])
	}
}

func TestLogLevelFiltering(t *testing.T) {
	os.Setenv("LUI_LOG_FORMAT", "text")
	defer os.Unsetenv("LUI_LOG_FORMAT")

	var buf bytes.Buffer
	InitLogger("error")
	slogger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	Debug("should not appear")
	Info("should not appear")
	Warning("should not appear")
	Error("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Fatalf("expected filtered messages not to appear, got: %s", output)
	}
	if !strings.Contains(output, "should appear") {
		t.Fatalf("expected 'should appear' in output, got: %s", output)
	}
}

func TestLevelFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"notice", slog.LevelInfo},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
	}

	for _, tt := range tests {
		result := levelFromString(tt.input)
		if result != tt.expected {
			t.Errorf("levelFromString(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestGetLogs(t *testing.T) {
	logBuffer = nil

	Info("info message")
	Warning("warning message")
	Error("error message")

	logs := GetLogs(10, "info")
	if len(logs) == 0 {
		t.Fatal("expected logs, got none")
	}
	found := false
	for _, log := range logs {
		if strings.Contains(log, "info message") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'info message' in logs, got: %v", logs)
	}
}

func TestGetLogsLevelFiltering(t *testing.T) {
	logBuffer = nil

	Debug("debug message")
	Info("info message")
	Warning("warning message")
	Error("error message")

	logs := GetLogs(10, "warning")
	for _, log := range logs {
		if strings.Contains(log, "debug message") || strings.Contains(log, "info message") {
			t.Fatalf("expected only warning+ level logs, got: %v", logs)
		}
	}
}
