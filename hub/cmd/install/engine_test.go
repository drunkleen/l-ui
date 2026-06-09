package install

import (
	"archive/tar"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── Helpers ────────────────────────────────────────────────────────

func makeTarball(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "test.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tarball: %v", err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0755,
			ModTime:  time.Now(),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gz: %v", err)
	}
	return path
}

func writeBinary(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho mock"), 0755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
}

// ── Tests ──────────────────────────────────────────────────────────

func TestExtractTarball(t *testing.T) {
	tmp := t.TempDir()
	tarball := makeTarball(t, tmp, map[string]string{
		"l-ui-hub/l-ui":         "binary-content",
		"l-ui-hub/l-ui.service": "[Unit]\nDescription=l-ui",
	})
	e := &Engine{
		cfg:     InstallConfig{Tarball: tarball, Port: "2053"},
		destDir: filepath.Join(tmp, "dest", "l-ui-hub"),
	}
	if err := e.extractTarball(); err != nil {
		t.Fatalf("extractTarball: %v", err)
	}
	// Verify binary exists
	bin := e.binaryPath()
	if _, err := os.Stat(bin); os.IsNotExist(err) {
		t.Fatalf("binary not found at %s", bin)
	}
	// Verify service file
	svc := filepath.Join(e.destDir, "l-ui.service")
	if _, err := os.Stat(svc); os.IsNotExist(err) {
		t.Fatalf("service file not found at %s", svc)
	}
}

func TestExtractTarballMissingBinary(t *testing.T) {
	tmp := t.TempDir()
	// Tarball without the binary
	tarball := makeTarball(t, tmp, map[string]string{
		"l-ui-hub/l-ui.service": "[Unit]",
	})
	e := &Engine{
		cfg:     InstallConfig{Tarball: tarball},
		destDir: filepath.Join(tmp, "dest", "l-ui-hub"),
	}
	if err := e.extractTarball(); err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestExtractTarballNoTarball(t *testing.T) {
	e := &Engine{
		cfg:     InstallConfig{},
		destDir: "/tmp/nonexistent",
	}
	if err := e.extractTarball(); err == nil {
		t.Fatal("expected error for no tarball")
	}
}

func TestBackupExisting(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "l-ui")
	os.MkdirAll(dest, 0755)
	os.WriteFile(filepath.Join(dest, "test.txt"), []byte("hello"), 0644)

	e := &Engine{destDir: dest}
	if err := e.backupExisting(); err != nil {
		t.Fatalf("backupExisting: %v", err)
	}
	if e.backup == "" {
		t.Fatal("backup path should be set")
	}
	if _, err := os.Stat(e.backup); os.IsNotExist(err) {
		t.Fatalf("backup dir not found: %s", e.backup)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("original dir should be gone after backup")
	}

	// Verify backup content
	content, _ := os.ReadFile(filepath.Join(e.backup, "test.txt"))
	if string(content) != "hello" {
		t.Fatalf("backup content = %q, want 'hello'", string(content))
	}
}

func TestBackupExistingFreshInstall(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "l-ui") // doesn't exist
	e := &Engine{destDir: dest}
	if err := e.backupExisting(); err != nil {
		t.Fatalf("backupExisting on fresh install: %v", err)
	}
	if e.backup != "" {
		t.Fatal("backup should be empty for fresh install")
	}
}

func TestRestoreBackup(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "l-ui")
	os.MkdirAll(dest, 0755)
	os.WriteFile(filepath.Join(dest, "test.txt"), []byte("hello"), 0644)

	e := &Engine{destDir: dest}
	if err := e.backupExisting(); err != nil {
		t.Fatalf("backupExisting: %v", err)
	}
	t.Logf("backup path: %s", e.backup)

	// Verify backup exists with content
	if _, err := os.Stat(e.backup); os.IsNotExist(err) {
		t.Fatalf("backup dir doesn't exist: %s", e.backup)
	}
	content, _ := os.ReadFile(filepath.Join(e.backup, "test.txt"))
	if string(content) != "hello" {
		t.Fatalf("backup content = %q, want 'hello'", string(content))
	}

	// Simulate new install
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	os.WriteFile(filepath.Join(dest, "new.txt"), []byte("new"), 0644)

	e.restoreBackup()

	// Check restored dir
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Fatal("restored dir should exist")
	}
	// Original content should be restored
	if _, err := os.Stat(filepath.Join(dest, "test.txt")); os.IsNotExist(err) {
		// Debug: list what's in dest
		entries, _ := os.ReadDir(dest)
		t.Logf("dest contents after restore:")
		for _, e := range entries {
			t.Logf("  %s", e.Name())
		}
		t.Fatal("backup content 'test.txt' not restored")
	}
	// New content should be gone
	if _, err := os.Stat(filepath.Join(dest, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("new content 'new.txt' should be gone after restore")
	}
	if e.backup != "" {
		t.Fatal("backup should be cleared after restore")
	}
}

func TestHealthCheckPasses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	done := make(chan bool, 1)
	go func() {
		resp, err := http.Get(server.URL)
		if err == nil && resp.StatusCode == http.StatusOK {
			done <- true
		}
	}()
	select {
	case <-done:
		// pass
	case <-time.After(3 * time.Second):
		t.Fatal("health check timed out")
	}
}

func TestHealthCheckFails(t *testing.T) {
	e := &Engine{cfg: InstallConfig{Port: "20999"}} // nothing listening
	err := e.healthCheckWithTimeout(1) // short timeout for test
	if err == nil {
		t.Fatal("expected health check to fail")
	}
}

func TestResult(t *testing.T) {
	e := &Engine{
		cfg: InstallConfig{
			Port:     "8080",
			BasePath: "/mypanel/",
			SSLType:  sslNone,
			Username: "admin",
			Password: "secret",
		},
		destDir: "/usr/local/l-ui-hub",
	}
	r := e.result()
	if r == nil {
		t.Fatal("result should not be nil")
	}
	if r.Username != "admin" {
		t.Errorf("username = %q, want 'admin'", r.Username)
	}
	if r.Password != "secret" {
		t.Errorf("password = %q, want 'secret'", r.Password)
	}
	if r.ConfigDir != "/usr/local/l-ui-hub" {
		t.Errorf("config dir = %q, want '/usr/local/l-ui-hub'", r.ConfigDir)
	}
}

func TestRandomPassLength(t *testing.T) {
	pw := randomString(24)
	if len(pw) != 24 {
		t.Fatalf("randomString(24) length = %d, want 24", len(pw))
	}
	pw8 := randomString(8)
	if len(pw8) != 8 {
		t.Fatalf("randomString(8) length = %d, want 8", len(pw8))
	}
}

func TestRandomPassCharset(t *testing.T) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	pw := randomString(100)
	for _, c := range pw {
		if !strings.ContainsRune(chars, c) {
			t.Fatalf("randomString contains invalid char %q", c)
		}
	}
}

func TestRandomPassUnique(t *testing.T) {
	pw1 := randomString(24)
	pw2 := randomString(24)
	if pw1 == pw2 {
		t.Fatal("two randomString calls produced the same password")
	}
}

func TestRandomPassNotEmpty(t *testing.T) {
	if randomString(0) != "" {
		t.Fatal("randomString(0) should return empty string")
	}
}

func TestResultWithSSL(t *testing.T) {
	e := &Engine{
		cfg: InstallConfig{
			Port:    "443",
			SSLType: sslDomain,
			Domain:  "example.com",
		},
	}
	r := e.result()
	if r == nil {
		t.Fatal("result should not be nil")
	}
	if !strings.Contains(r.AccessURL, "https://") {
		t.Errorf("expected https in URL, got %s", r.AccessURL)
	}
}
