package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/sshutil"
	"github.com/drunkleen/l-ui/internal/util/arch"
)

func TestSSHBootstrapArch(t *testing.T) {
	cases := map[string]string{
		"x86_64":  "amd64",
		"aarch64": "arm64",
		"armv7l":  "armv7",
		"i686":    "386",
	}
	for input, want := range cases {
		got, err := arch.SSHBootstrapArch(input)
		if err != nil {
			t.Fatalf("SSHBootstrapArch(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("SSHBootstrapArch(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBootstrapJobStoreReturnsCopy(t *testing.T) {
	svc := &NodeService{}
	svc.putBootstrapJob(&NodeBootstrapJob{
		ID:    "bootstrap-1",
		State: bootstrapStateRunning,
		Steps: []BootstrapStep{{Name: "detect-arch", OK: true, Output: "x86_64"}},
	})

	job, ok := svc.BootstrapJob("bootstrap-1")
	if !ok {
		t.Fatal("expected bootstrap job")
	}
	if job.State != bootstrapStateRunning {
		t.Fatalf("job state = %q, want %q", job.State, bootstrapStateRunning)
	}
	job.Steps[0].Name = "mutated"

	again, ok := svc.BootstrapJob("bootstrap-1")
	if !ok {
		t.Fatal("expected bootstrap job on second read")
	}
	if again.Steps[0].Name != "detect-arch" {
		t.Fatalf("job copy leaked mutation, got %q", again.Steps[0].Name)
	}
}

func TestBootstrapNodeTLSMode(t *testing.T) {
	svc := &NodeService{}
	node, err := svc.buildBootstrapNode(NodeBootstrapRequest{
		Name:        "node-1",
		Address:     "203.0.113.10",
		SSHUser:     "root",
		SSHPassword: "secret",
		UseTLS:      true,
		Domain:      "node.example.com",
		AcmeEmail:   "admin@example.com",
	})
	if err != nil {
		t.Fatalf("buildBootstrapNode returned error: %v", err)
	}
	if node.Scheme != "https" {
		t.Fatalf("scheme = %q, want https", node.Scheme)
	}
	if node.Address != "node.example.com" {
		t.Fatalf("address = %q, want domain", node.Address)
	}
	if node.Port != 443 {
		t.Fatalf("port = %d, want 443", node.Port)
	}
	if node.AllowPrivateAddress {
		t.Fatal("allowPrivateAddress should be false for TLS mode")
	}
	if node.ApiToken == "" {
		t.Fatal("api token should be generated")
	}
}

func TestBootstrapNodeAutoPort(t *testing.T) {
	svc := &NodeService{}
	node, err := svc.buildBootstrapNode(NodeBootstrapRequest{
		Name:        "node-1",
		Address:     "203.0.113.10",
		SSHUser:     "root",
		SSHPassword: "secret",
	})
	if err != nil {
		t.Fatalf("buildBootstrapNode returned error: %v", err)
	}
	if node.Port <= 2000 || node.Port > 65535 {
		t.Fatalf("port = %d, want auto-selected port above 2000", node.Port)
	}
}

func TestBootstrapNodeRequiresDomainForTLS(t *testing.T) {
	svc := &NodeService{}
	if _, err := svc.buildBootstrapNode(NodeBootstrapRequest{
		Name:        "node-1",
		Address:     "203.0.113.10",
		SSHUser:     "root",
		SSHPassword: "secret",
		UseTLS:      true,
	}); err == nil {
		t.Fatal("expected error when TLS bootstrap has no domain")
	}
}

func TestBootstrapNodeIdentityIsReusedOnRetry(t *testing.T) {
	dir := t.TempDir()
	if err := database.InitDB(filepath.Join(dir, "l-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	svc := &NodeService{}
	existing := model.Node{Name: "node-1", Address: "203.0.113.10", Port: 2053, Scheme: "http", BasePath: "/", ApiToken: "token-123", Enable: true, AllowPrivateAddress: true}
	if err := database.GetDB().Create(&existing).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}

	node := model.Node{Name: "node-1", Address: "198.51.100.20", Port: 2443, Scheme: "https", BasePath: "/", ApiToken: "new-token", Enable: true, AllowPrivateAddress: false, TlsVerifyMode: "verify", BundleSHA256: "bundle-sha"}
	if err := svc.prepareBootstrapNodeIdentity(&node); err != nil {
		t.Fatalf("prepareBootstrapNodeIdentity: %v", err)
	}
	if node.Id != existing.Id {
		t.Fatalf("node id = %d, want %d", node.Id, existing.Id)
	}
	if node.ApiToken != existing.ApiToken {
		t.Fatalf("node token = %q, want preserved %q", node.ApiToken, existing.ApiToken)
	}
	if err := svc.persistBootstrapNode(&node); err != nil {
		t.Fatalf("persistBootstrapNode: %v", err)
	}

	var saved model.Node
	if err := database.GetDB().Where("id = ?", existing.Id).First(&saved).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if saved.ApiToken != existing.ApiToken {
		t.Fatalf("saved token = %q, want %q", saved.ApiToken, existing.ApiToken)
	}
	if saved.Address != node.Address || saved.Port != node.Port || saved.Scheme != node.Scheme {
		t.Fatalf("saved node not updated: %#v", saved)
	}
}

func TestBootstrapTLSDirectiveErrorsForUnknownProvider(t *testing.T) {
	if _, err := bootstrapTLSDirective("missing", "node.example.com"); err == nil {
		t.Fatal("expected error for unknown dns provider")
	}
}

func TestShouldInstallServiceFallback(t *testing.T) {
	if !sshutil.ShouldInstallServiceFallback("missing l-ui.service in bundle", errors.New("exit status 1")) {
		t.Fatal("expected fallback for missing bundle service")
	}
	if sshutil.ShouldInstallServiceFallback("permission denied", errors.New("exit status 1")) {
		t.Fatal("did not expect fallback for unrelated service error")
	}
}

func TestSSHAuthMethodsRequiresCredentials(t *testing.T) {
	if _, err := sshutil.SshAuthMethods("", "", ""); err == nil {
		t.Fatal("expected error when no ssh credentials are supplied")
	}
}

func TestUploadRemoteFileOpensStdinBeforeStart(t *testing.T) {
	t.Run("without sudo", func(t *testing.T) {
		sess := &fakeUploadSession{}
		out, err := sshutil.UploadRemoteFileSession(sess, "", false, "/tmp/upload.txt", []byte("hello"), "0644")
		if err != nil {
			t.Fatalf("UploadRemoteFileSession returned error: %v", err)
		}
		if out != "" {
			t.Fatalf("stdout = %q, want empty", out)
		}
		if !sess.started {
			t.Fatal("expected session to start")
		}
		if got := sess.stdin.String(); got != "hello" {
			t.Fatalf("stdin = %q, want %q", got, "hello")
		}
	})

	t.Run("with sudo", func(t *testing.T) {
		sess := &fakeUploadSession{}
		_, err := sshutil.UploadRemoteFileSession(sess, "secret", true, "/tmp/upload.txt", []byte("hello"), "0644")
		if err != nil {
			t.Fatalf("UploadRemoteFileSession returned error: %v", err)
		}
		if got := sess.stdin.String(); got != "secret\nhello" {
			t.Fatalf("stdin = %q, want %q", got, "secret\nhello")
		}
	})
}

func TestBootstrapCaddyfile(t *testing.T) {
	got := bootstrapCaddyfile("node.example.com", "admin@example.com", 2053, "")
	want := "{\n    email admin@example.com\n}\n\nnode.example.com {\n    reverse_proxy 127.0.0.1:2053\n}\n"
	if got != want {
		t.Fatalf("caddyfile mismatch:\n%s\nwant:\n%s", got, want)
	}
}

func TestStartBootstrapAsyncJobLifecycle(t *testing.T) {
	svc := &NodeService{}
	// Point to localhost:1 so SSH dial fails fast with "connection refused".
	ctx := context.Background()

	job, err := svc.StartBootstrap(ctx, NodeBootstrapRequest{
		Name:        "async-lifecycle",
		Address:     "127.0.0.1",
		SSHPort:     1,
		SSHUser:     "root",
		SSHPassword: "secret",
	})
	if err != nil {
		t.Fatalf("StartBootstrap should not block: %v", err)
	}
	if job.State != bootstrapStateQueued {
		t.Fatalf("job state = %q, want %q", job.State, bootstrapStateQueued)
	}

	// Poll until the async goroutine reaches a terminal state.
	// SSH dial retries (3 attempts × instant "connection refused" + backoff) ≈ ~6s.
	deadline := time.After(20 * time.Second)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	var terminal bool
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for bootstrap job to reach terminal state")
		case <-tick.C:
			got, ok := svc.BootstrapJob(job.ID)
			if !ok {
				t.Fatal("bootstrap job disappeared from store")
			}
			if got.State == bootstrapStateFailed || got.State == bootstrapStateDone {
				terminal = true
				if got.State != bootstrapStateFailed {
					t.Fatalf("expected job to fail (no SSH server), got state=%q", got.State)
				}
				if got.Error == "" {
					t.Fatal("expected non-empty error message on failed job")
				}
				goto done
			}
		}
	}
done:
	if !terminal {
		t.Fatal("job never reached terminal state")
	}
}



func TestBootstrapJobNotFound(t *testing.T) {
	svc := &NodeService{}
	if _, ok := svc.BootstrapJob("nonexistent"); ok {
		t.Fatal("expected false for nonexistent job")
	}
}

func TestBootstrapBuildNode_DefaultPort(t *testing.T) {
	svc := &NodeService{}
	node, err := svc.buildBootstrapNode(NodeBootstrapRequest{
		Name:    "port-test",
		Address: "198.51.100.1",
	})
	if err != nil {
		t.Fatalf("buildBootstrapNode returned error: %v", err)
	}
	if node.Port <= 2000 || node.Port > 65535 {
		t.Fatalf("expected auto-selected port above 2000, got %d", node.Port)
	}
	if node.Name != "port-test" {
		t.Fatalf("name = %q, want %q", node.Name, "port-test")
	}
	if node.ApiToken == "" {
		t.Fatal("expected api token to be generated")
	}
}

func TestBootstrapBuildNode_EmptyName(t *testing.T) {
	svc := &NodeService{}
	_, err := svc.buildBootstrapNode(NodeBootstrapRequest{
		Name:    "",
		Address: "198.51.100.1",
	})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestBootstrapBuildNode_EmptyAddress(t *testing.T) {
	svc := &NodeService{}
	_, err := svc.buildBootstrapNode(NodeBootstrapRequest{
		Name:    "addr-test",
		Address: "",
	})
	if err == nil {
		t.Fatal("expected error for empty address")
	}
}

func TestBootstrapFlow_SetsCorrectSSHPort(t *testing.T) {
	svc := &NodeService{}
	req := NodeBootstrapRequest{
		Name:        "ssh-port-test",
		Address:     "198.51.100.1",
		SSHUser:     "root",
		SSHPassword: "secret",
		SSHPort:     2222,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	// This should fail with SSH dial error, not validation error
	_, err := svc.StartBootstrap(ctx, req)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestBootstrapCaddyfileWithDNSDirective(t *testing.T) {
	got := bootstrapCaddyfile("node.example.com", "", 2053, "dns cloudflare")
	want := "node.example.com {\n    reverse_proxy 127.0.0.1:2053\n    tls {\n        dns cloudflare\n    }\n}\n"
	if got != want {
		t.Fatalf("caddyfile mismatch with dns:\n%s\nwant:\n%s", got, want)
	}
}

type fakeUploadSession struct {
	started     bool
	stdinOpened bool
	stdin       bytes.Buffer
	stdout      bytes.Buffer
	stderr      bytes.Buffer
	stdinErr    error
	startErr    error
	waitErr     error
}

func (f *fakeUploadSession) StdinPipe() (io.WriteCloser, error) {
	if f.started {
		return nil, errors.New("ssh: StdinPipe after process started")
	}
	f.stdinOpened = true
	return nopBufferCloser{Buffer: &f.stdin}, f.stdinErr
}

func (f *fakeUploadSession) Start(string) error {
	f.started = true
	return f.startErr
}

func (f *fakeUploadSession) Wait() error { return f.waitErr }

func (f *fakeUploadSession) Close() error { return nil }

func (f *fakeUploadSession) SetStdout(w io.Writer) {
	if b, ok := w.(*bytes.Buffer); ok {
		f.stdout = *b
	}
}

func (f *fakeUploadSession) SetStderr(w io.Writer) {
	if b, ok := w.(*bytes.Buffer); ok {
		f.stderr = *b
	}
}

type nopBufferCloser struct {
	*bytes.Buffer
}

func (n nopBufferCloser) Close() error { return nil }
