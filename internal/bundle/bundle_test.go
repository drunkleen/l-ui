package bundle

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFileToPath_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello bundle"))
	}))
	defer ts.Close()

	dst := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := DownloadFileToPath(ts.URL, dst); err != nil {
		t.Fatalf("DownloadFileToPath: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != "hello bundle" {
		t.Fatalf("content = %q, want %q", string(data), "hello bundle")
	}
}

func TestDownloadFileToPath_RetriesOnTransientFailure(t *testing.T) {
	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("hello after retry"))
	}))
	defer ts.Close()

	dst := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := DownloadFileToPath(ts.URL, dst); err != nil {
		t.Fatalf("DownloadFileToPath: %v (attempts=%d)", err, attempt)
	}
	if attempt < 3 {
		t.Fatalf("expected at least 3 attempts, got %d", attempt)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != "hello after retry" {
		t.Fatalf("content = %q, want %q", string(data), "hello after retry")
	}
}

func TestDownloadFileToPath_DoesNotRetryOn4xx(t *testing.T) {
	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	dst := filepath.Join(t.TempDir(), "bundle.tar.gz")
	err := DownloadFileToPath(ts.URL, dst)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if attempt != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempt)
	}
}

func TestDownloadFileToPath_DoesNotRetryOnBadGateway(t *testing.T) {
	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintln(w, "upstream failure")
	}))
	defer ts.Close()

	dst := filepath.Join(t.TempDir(), "bundle.tar.gz")
	err := DownloadFileToPath(ts.URL, dst)
	if err == nil {
		t.Fatal("expected error for 502")
	}
	// 502 is a 5xx → should be transient, so more than 1 attempt
	if attempt <= 1 {
		t.Fatalf("expected retries for 502, got %d attempt(s)", attempt)
	}
}

func TestDownloadFileToPath_HandlesNetworkError(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "bundle.tar.gz")
	err := DownloadFileToPath("http://127.0.0.1:1/nonexistent", dst)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}
