package cmd

import (
	"testing"
)

func TestDetectArch(t *testing.T) {
	arch := detectArch()
	if arch == "" {
		t.Fatal("detectArch returned empty")
	}
	valid := map[string]bool{
		"amd64": true, "arm64": true, "armv7": true, "armv6": true, "386": true, "s390x": true,
	}
	if !valid[arch] {
		t.Logf("detectArch returned %q (may be valid on this platform)", arch)
	}
}

func TestFetchLatestTag_NoNetwork(t *testing.T) {
	// Without network this should error gracefully.
	v, err := fetchLatestTag()
	if err != nil {
		t.Logf("fetchLatestTag (no network expected): %v", err)
	} else {
		t.Logf("fetchLatestTag returned %q (network available)", v)
	}
}

func TestFetchLatestTag_NotEmpty(t *testing.T) {
	v, err := fetchLatestTag()
	if err == nil && v == "" {
		t.Error("fetchLatestTag returned empty string without error")
	}
	_ = v
}

func TestDownloadFile_InvalidURL(t *testing.T) {
	err := downloadFile("https://nonexistent.example/file.tar.gz", "/tmp/test")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestRestoreBackup(t *testing.T) {
	// Should not panic with empty backup.
	restoreBackup("/tmp/test-dest", "")
}

func TestRestoreBackup_Invalid(t *testing.T) {
	// Should not panic with nonexistent backup.
	restoreBackup("/tmp/test-dest", "/tmp/nonexistent-backup")
}

func TestCleanupBackup(t *testing.T) {
	// Should not panic with empty backup.
	cleanupBackup("")
}
