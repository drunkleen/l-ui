package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogService_New(t *testing.T) {
	svc := NewLogService()
	if svc == nil {
		t.Fatal("expected non-nil LogService")
	}
}

func TestLogService_TailLog_ReturnsLastNLines(t *testing.T) {
	svc := NewLogService()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	lines := make([]string, 100)
	for i := range 100 {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, err := svc.TailLog(logFile, 10)
	if err != nil {
		t.Fatalf("TailLog failed: %v", err)
	}
	if len(result) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(result))
	}
}

func TestLogService_TailLog_ReturnsAllWhenShorterThanTail(t *testing.T) {
	svc := NewLogService()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "short.log")

	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, err := svc.TailLog(logFile, 50)
	if err != nil {
		t.Fatalf("TailLog failed: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(result))
	}
}

func TestLogService_TailLog_DefaultTailLines(t *testing.T) {
	svc := NewLogService()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "default.log")

	lines := make([]string, 60)
	for i := range 60 {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, err := svc.TailLog(logFile, 0)
	if err != nil {
		t.Fatalf("TailLog failed: %v", err)
	}
	if len(result) != 50 {
		t.Fatalf("expected 50 lines (default), got %d", len(result))
	}
}

func TestLogService_TailLog_ClampsToMax(t *testing.T) {
	svc := NewLogService()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "max.log")

	lines := make([]string, 1000)
	for i := range 1000 {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, err := svc.TailLog(logFile, 9999)
	if err != nil {
		t.Fatalf("TailLog failed: %v", err)
	}
	if len(result) > 500 {
		t.Fatalf("expected at most 500 lines, got %d", len(result))
	}
	if len(result) != 500 {
		t.Fatalf("expected exactly 500 lines (max), got %d", len(result))
	}
}

func TestLogService_TailLog_FileNotFound(t *testing.T) {
	svc := NewLogService()
	_, err := svc.TailLog("/tmp/nonexistent/foo.log", 50)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLogService_TailLog_EmptyFile(t *testing.T) {
	svc := NewLogService()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, err := svc.TailLog(logFile, 10)
	if err != nil {
		t.Fatalf("TailLog failed: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 lines for empty file, got %d", len(result))
	}
}

func TestLogService_TailLog_NoTrailingNewline(t *testing.T) {
	svc := NewLogService()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "notrailing.log")
	content := "line1\nline2\nline3"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, err := svc.TailLog(logFile, 2)
	if err != nil {
		t.Fatalf("TailLog failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	if result[0] != "line2" || result[1] != "line3" {
		t.Fatalf("expected last 2 lines, got %v", result)
	}
}

func TestLogService_TailLog_NegativeTail(t *testing.T) {
	svc := NewLogService()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "neg.log")
	if err := os.WriteFile(logFile, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, err := svc.TailLog(logFile, -5)
	if err != nil {
		t.Fatalf("TailLog failed: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected all 3 lines when tail <= 0, got %d", len(result))
	}
}
