package controller

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestLogsController_TailLog_MissingPath(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/logs", "")
	ctrl := &LogsController{}
	ctrl.TailLog(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing path, got %d", w.Code)
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] == "" {
		t.Fatal("expected error message")
	}
}

func TestLogsController_TailLog_ValidFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	lines := []byte("line1\nline2\nline3\nline4\nline5\n")
	if err := os.WriteFile(logFile, lines, 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	c, w := newTestContext("GET", "/api/v1/logs?path="+logFile+"&lines=3", "")
	ctrl := &LogsController{}
	ctrl.TailLog(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool   `json:"success"`
		Obj     string `json:"obj"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Obj != "line3\nline4\nline5" {
		t.Fatalf("expected 3 lines, got %q", resp.Obj)
	}
}

func TestLogsController_TailLog_DefaultLines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "default.log")
	var content []byte
	for range 100 {
		content = append(content, "line\n"...)
	}
	if err := os.WriteFile(logFile, content, 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	c, w := newTestContext("GET", "/api/v1/logs?path="+logFile, "")
	ctrl := &LogsController{}
	ctrl.TailLog(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Success bool   `json:"success"`
		Obj     string `json:"obj"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Obj == "" {
		t.Fatal("expected non-empty obj")
	}
}

func TestLogsController_TailLog_ClampsToMax(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "max.log")
	var content []byte
	for range 1000 {
		content = append(content, "line\n"...)
	}
	if err := os.WriteFile(logFile, content, 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	c, w := newTestContext("GET", "/api/v1/logs?path="+logFile+"&lines=9999", "")
	ctrl := &LogsController{}
	ctrl.TailLog(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Success bool   `json:"success"`
		Obj     string `json:"obj"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
}

func TestLogsController_TailLog_NonexistentFile(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/logs?path=/tmp/nonexistent-12345.log&lines=10", "")
	ctrl := &LogsController{}
	ctrl.TailLog(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for nonexistent file, got %d", w.Code)
	}
}

func TestLogsController_TailLog_InvalidLinesParam(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "invalid-lines.log")
	if err := os.WriteFile(logFile, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	c, w := newTestContext("GET", "/api/v1/logs?path="+logFile+"&lines=invalid", "")
	ctrl := &LogsController{}
	ctrl.TailLog(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (defaults on invalid lines), got %d", w.Code)
	}
	var resp struct {
		Success bool   `json:"success"`
		Obj     string `json:"obj"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Obj != "a\nb\nc" {
		t.Fatalf("expected 3 lines, got %q", resp.Obj)
	}
}

func TestLogsController_TailLog_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	c, w := newTestContext("GET", "/api/v1/logs?path="+logFile+"&lines=10", "")
	ctrl := &LogsController{}
	ctrl.TailLog(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Success bool   `json:"success"`
		Obj     string `json:"obj"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Obj != "" {
		t.Fatalf("expected empty obj, got %q", resp.Obj)
	}
}
