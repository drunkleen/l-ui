package controller

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestLogsController_TailLog_MissingPath(t *testing.T) {
	ctrl := &LogsController{}

	app := fiber.New()
	app.Get("/api/v1/logs", ctrl.TailLog)

	resp, err := app.Test(testRequest("GET", "/api/v1/logs", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing path, got %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["error"] == "" {
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

	ctrl := &LogsController{}

	app := fiber.New()
	app.Get("/api/v1/logs", ctrl.TailLog)

	resp, err := app.Test(testRequest("GET", "/api/v1/logs?path="+logFile+"&lines=3", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	var result struct {
		Success bool   `json:"success"`
		Obj     string `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if result.Obj != "line3\nline4\nline5" {
		t.Fatalf("expected 3 lines, got %q", result.Obj)
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

	ctrl := &LogsController{}

	app := fiber.New()
	app.Get("/api/v1/logs", ctrl.TailLog)

	resp, err := app.Test(testRequest("GET", "/api/v1/logs?path="+logFile, ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result struct {
		Success bool   `json:"success"`
		Obj     string `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if result.Obj == "" {
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

	ctrl := &LogsController{}

	app := fiber.New()
	app.Get("/api/v1/logs", ctrl.TailLog)

	resp, err := app.Test(testRequest("GET", "/api/v1/logs?path="+logFile+"&lines=9999", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result struct {
		Success bool   `json:"success"`
		Obj     string `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
}

func TestLogsController_TailLog_NonexistentFile(t *testing.T) {
	ctrl := &LogsController{}

	app := fiber.New()
	app.Get("/api/v1/logs", ctrl.TailLog)

	resp, err := app.Test(testRequest("GET", "/api/v1/logs?path=/tmp/nonexistent-12345.log&lines=10", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 for nonexistent file, got %d", resp.StatusCode)
	}
}

func TestLogsController_TailLog_InvalidLinesParam(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "invalid-lines.log")
	if err := os.WriteFile(logFile, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	ctrl := &LogsController{}

	app := fiber.New()
	app.Get("/api/v1/logs", ctrl.TailLog)

	resp, err := app.Test(testRequest("GET", "/api/v1/logs?path="+logFile+"&lines=invalid", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (defaults on invalid lines), got %d", resp.StatusCode)
	}
	var result struct {
		Success bool   `json:"success"`
		Obj     string `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Obj != "a\nb\nc" {
		t.Fatalf("expected 3 lines, got %q", result.Obj)
	}
}

func TestLogsController_TailLog_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	ctrl := &LogsController{}

	app := fiber.New()
	app.Get("/api/v1/logs", ctrl.TailLog)

	resp, err := app.Test(testRequest("GET", "/api/v1/logs?path="+logFile+"&lines=10", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result struct {
		Success bool   `json:"success"`
		Obj     string `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Obj != "" {
		t.Fatalf("expected empty obj, got %q", result.Obj)
	}
}
