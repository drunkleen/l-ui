package install

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// InstallConfig holds all settings collected by the TUI or CLI flags.
type InstallConfig struct {
	Port     string
	BasePath string
	SSLType  sslType
	Domain   string
	IP       string
	CertPath string
	Username string
	Password string
	Tarball  string
	Version  string
}

// InstallResult is returned after a successful installation.
type InstallResult struct {
	AccessURL string
	Username  string
	Password  string
	ConfigDir string
	LogFile   string
}

// Engine handles the actual installation steps.
type Engine struct {
	cfg      InstallConfig
	destDir  string
	backup   string
	svcMgr   string
	log      *log.Logger
	logFile  string
	progress func(string)
}

// ProgressFunc is called before each installation step with the step name.
type ProgressFunc func(string)

// NewEngine creates an install engine from config.
func NewEngine(cfg InstallConfig, progress ...ProgressFunc) *Engine {
	dest := "/usr/local/l-ui-hub"
	logFile := filepath.Join(os.TempDir(), fmt.Sprintf("l-ui-install-%d.log", time.Now().Unix()))
	f, err := os.Create(logFile)
	l := log.New(os.Stderr, "", 0)
	if err == nil {
		l = log.New(f, "", log.LstdFlags)
	}
	e := &Engine{
		cfg:     cfg,
		destDir: dest,
		svcMgr:  detectInit(),
		log:     l,
		logFile: logFile,
	}
	if len(progress) > 0 {
		e.progress = progress[0]
	}
	return e
}

func detectInit() string {
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return "systemd"
	}
	if _, err := os.Stat("/sbin/openrc-run"); err == nil {
		return "openrc"
	}
	return "unknown"
}

// Run executes the full installation.
func (e *Engine) Run() (*InstallResult, error) {
	e.log.Println("=== L-UI Installation Log ===")
	e.log.Printf("Config: port=%s basePath=%s ssl=%d\n", e.cfg.Port, e.cfg.BasePath, e.cfg.SSLType)

	steps := []struct {
		name string
		fn   func() error
	}{
		{"stop service", e.stopService},
		{"backup existing", e.backupExisting},
		{"extract tarball", e.extractTarball},
		{"apply settings", e.applySettings},
		{"install symlink", e.installSymlink},
		{"install service file", e.installServiceFile},
		{"configure SSL", e.configureSSL},
		{"start service", e.startService},
		{"health check", e.healthCheck},
	}

	for _, s := range steps {
		if e.progress != nil {
			e.progress(s.name)
		}
		e.log.Printf("→ %s\n", s.name)
		if err := s.fn(); err != nil {
			e.log.Printf("✗ %s failed: %s\n", s.name, err)
			if s.name == "extract tarball" || s.name == "apply settings" || s.name == "health check" {
				e.restoreBackup()
			}
			return nil, &installError{Step: s.name, Err: err, LogFile: e.logFile}
		}
		e.log.Printf("✓ %s\n", s.name)
	}

	r := e.result()
	r.LogFile = e.logFile
	return r, nil
}

type installError struct {
	Step    string
	Err     error
	LogFile string
}

func (e *installError) Error() string {
	return fmt.Sprintf("%s failed: %s\nLog: %s", e.Step, e.Err, e.LogFile)
}

func (e *Engine) binaryPath() string { return filepath.Join(e.destDir, "l-ui") }

// ── Steps ───────────────────────────────────────────────────────────

func (e *Engine) stopService() error {
	switch e.svcMgr {
	case "systemd":
		exec.Command("systemctl", "stop", "l-ui").Run()
	case "openrc":
		exec.Command("rc-service", "l-ui", "stop").Run()
	}
	return nil
}

func (e *Engine) backupExisting() error {
	if _, err := os.Stat(e.destDir); os.IsNotExist(err) {
		return nil
	}
	backup := e.destDir + ".bak." + fmt.Sprintf("%d", time.Now().Unix())
	if err := os.Rename(e.destDir, backup); err != nil {
		return err
	}
	e.backup = backup
	return nil
}

func (e *Engine) restoreBackup() {
	if e.backup == "" {
		return
	}
	_ = os.RemoveAll(e.destDir)
	if err := os.Rename(e.backup, e.destDir); err != nil {
		e.log.Printf("restore backup: %v\n", err)
	}
	e.backup = ""
}

func (e *Engine) extractTarball() error {
	tarball := e.cfg.Tarball
	if tarball == "" {
		return fmt.Errorf("no tarball path provided")
	}
	if err := os.MkdirAll(e.destDir, 0755); err != nil {
		return err
	}
	cmd := exec.Command("tar", "-xzf", tarball, "-C", filepath.Dir(e.destDir))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%s", err.Error(), strings.TrimSpace(string(out)))
	}
	bin := e.binaryPath()
	if _, err := os.Stat(bin); os.IsNotExist(err) {
		return fmt.Errorf("binary not found at %s after extraction", bin)
	}
	if err := os.Chmod(bin, 0755); err != nil {
		return err
	}
	return nil
}

func (e *Engine) applySettings() error {
	bin := e.binaryPath()
	// Run username+password together — updateSetting requires both
	cmds := [][]string{
		{bin, "setting", "-port", e.cfg.Port},
		{bin, "setting", "-webBasePath", e.cfg.BasePath},
		{bin, "setting", "-username", e.cfg.Username, "-password", e.cfg.Password},
	}
	for _, args := range cmds {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %s", err.Error(), strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func (e *Engine) installSymlink() error {
	link := "/usr/local/bin/l-ui"
	_ = os.Remove(link)
	if err := os.Symlink(e.binaryPath(), link); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}
	return nil
}

func (e *Engine) installServiceFile() error {
	if e.svcMgr != "systemd" {
		return nil
	}
	svcSrc := filepath.Join(e.destDir, "l-ui.service")
	svcDst := "/etc/systemd/system/l-ui.service"

	// Try service file variants
	for _, name := range []string{"l-ui.service", "l-ui.service.debian", "l-ui.service.rhel", "l-ui.service.arch"} {
		src := filepath.Join(e.destDir, name)
		if _, err := os.Stat(src); err == nil {
			svcSrc = src
			break
		}
	}
	data, err := os.ReadFile(svcSrc)
	if err != nil {
		return fmt.Errorf("service file not found in bundle: %w", err)
	}
	// Patch ExecStart to include 'run' subcommand if missing
	content := string(data)
	if !bytes.Contains(data, []byte("/usr/local/l-ui-hub/l-ui run")) {
		content = strings.ReplaceAll(content, "ExecStart=/usr/local/l-ui-hub/l-ui", "ExecStart=/usr/local/l-ui-hub/l-ui run")
	}
	if err := os.WriteFile(svcDst, []byte(content), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}
	exec.Command("chown", "root:root", svcDst).Run()
	e.log.Printf("installed service file: %s\n", svcDst)
	return nil
}

func (e *Engine) installAcmeSh() error {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/root"
	}
	acmePath := filepath.Join(home, ".acme.sh", "acme.sh")
	if _, err := os.Stat(acmePath); err == nil {
		return nil // already installed
	}
	e.log.Println("installing acme.sh...")
	cmd := exec.Command("curl", "-fsSL", "https://get.acme.sh", "-o", "/tmp/acme.sh")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("download acme.sh: %w\n%s", err, out)
	}
	install := exec.Command("sh", "/tmp/acme.sh")
	if out, err := install.CombinedOutput(); err != nil {
		return fmt.Errorf("install acme.sh: %w\n%s", err, out)
	}
	// Set default CA to Let's Encrypt (ZeroSSL has rate limiting issues)
	exec.Command(acmePath, "--set-default-ca", "--server", "letsencrypt").Run()
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func (e *Engine) issueCert(domain, ip string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/root"
	}
	acmePath := filepath.Join(home, ".acme.sh", "acme.sh")

	var args []string
	var certDir string
	if domain != "" {
		// Use standalone mode (port 80) with Let's Encrypt RSA certs.
		// --keylength 2048 ensures RSA certs stored in <domain>/ not <domain>_ecc/
		args = []string{"--issue", "--standalone", "-d", domain, "--server", "letsencrypt", "--keylength", "2048"}
		certDir = filepath.Join(home, ".acme.sh", domain)
	} else if ip != "" {
		args = []string{"--issue", "--standalone", "-d", ip, "--ip", ip, "--server", "letsencrypt", "--keylength", "2048"}
		certDir = filepath.Join(home, ".acme.sh", ip)
	} else {
		return "", fmt.Errorf("either domain or IP required for SSL")
	}

	// ── Check if cert already exists and is valid ──────────────
	if _, err := tls.LoadX509KeyPair("/etc/l-ui/cert/fullchain.pem", "/etc/l-ui/cert/privkey.pem"); err == nil && os.Getenv("LUI_FORCE_RENEW") == "" {
		e.log.Println("certificate already exists and is valid at /etc/l-ui/cert/, skipping acme.sh")
		return "/etc/l-ui/cert", nil
	}
	if _, err := os.Stat("/etc/l-ui/cert/fullchain.pem"); err == nil {
		e.log.Println("existing certificate is invalid, re-issuing...")
		os.Remove("/etc/l-ui/cert/fullchain.pem")
		os.Remove("/etc/l-ui/cert/privkey.pem")
	}

	// Stop panel temporarily to free port 80
	exec.Command("systemctl", "stop", "l-ui").Run()

	out, err := exec.Command(acmePath, args...).CombinedOutput()
	output := string(out)
	if err != nil {
		// "Skipping" or "Domains not changed" means cert already exists and is valid
		if strings.Contains(output, "Skipping") || strings.Contains(output, "Domains not changed") {
			e.log.Println("acme.sh: certificate already valid, using existing")
		} else {
			return "", fmt.Errorf("acme.sh failed:\n%s", output)
		}
	}

	// acme.sh stores RSA certs at ~/.acme.sh/<domain>/
	fullchain := filepath.Join(certDir, "fullchain.cer")
	key := filepath.Join(certDir, domain+".key")
	// Fallback to PEM extension
	if _, err := os.Stat(fullchain); os.IsNotExist(err) {
		fullchain = filepath.Join(certDir, "fullchain.pem")
	}
	if _, err := os.Stat(key); os.IsNotExist(err) {
		key = filepath.Join(certDir, domain+".key")
	}
	// Verify cert files exist
	if _, err := os.Stat(fullchain); os.IsNotExist(err) {
		return "", fmt.Errorf("certificate not found at %s — acme.sh output above may explain why", fullchain)
	}
	return certDir, nil
}

func (e *Engine) configureSSL() error {
	if e.cfg.SSLType == sslNone {
		return nil
	}

	certDir := "/etc/l-ui/cert"
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}

	bin := e.binaryPath()
	switch e.cfg.SSLType {
	case sslDomain:
		if e.cfg.Domain == "" {
			return fmt.Errorf("domain required for SSL")
		}
		e.log.Printf("issuing SSL certificate for domain: %s\n", e.cfg.Domain)
		if err := e.installAcmeSh(); err != nil {
			return err
		}
		srcDir, err := e.issueCert(e.cfg.Domain, "")
		if err != nil {
			return fmt.Errorf("SSL issue: %w", err)
		}
		if srcDir != certDir {
			copyFile(filepath.Join(srcDir, "fullchain.cer"), filepath.Join(certDir, "fullchain.pem"))
			copyFile(filepath.Join(srcDir, e.cfg.Domain+".key"), filepath.Join(certDir, "privkey.pem"))
		}

	case sslIP:
		if e.cfg.IP == "" {
			return fmt.Errorf("IP required for SSL")
		}
		e.log.Printf("issuing SSL certificate for IP: %s\n", e.cfg.IP)
		if err := e.installAcmeSh(); err != nil {
			return err
		}
		srcDir, err := e.issueCert("", e.cfg.IP)
		if err != nil {
			return fmt.Errorf("SSL issue-ip: %w", err)
		}
		if srcDir != certDir {
			copyFile(filepath.Join(srcDir, "fullchain.cer"), filepath.Join(certDir, "fullchain.pem"))
			copyFile(filepath.Join(srcDir, e.cfg.IP+".key"), filepath.Join(certDir, "privkey.pem"))
		}

	case sslCustom:
		if e.cfg.CertPath == "" {
			return fmt.Errorf("cert path required for custom SSL")
		}
		copyFile(filepath.Join(e.cfg.CertPath, "fullchain.pem"), filepath.Join(certDir, "fullchain.pem"))
		copyFile(filepath.Join(e.cfg.CertPath, "privkey.pem"), filepath.Join(certDir, "privkey.pem"))
	}

	// Verify cert files
	if _, err := os.Stat(filepath.Join(certDir, "fullchain.pem")); os.IsNotExist(err) {
		return fmt.Errorf("certificate file not found after SSL setup")
	}
	if _, err := os.Stat(filepath.Join(certDir, "privkey.pem")); os.IsNotExist(err) {
		return fmt.Errorf("key file not found after SSL setup")
	}

	// Configure panel to use the certificate (using 'cert' subcommand, not 'setting')
	certCmds := [][]string{
		{bin, "cert", "-webCert", filepath.Join(certDir, "fullchain.pem"), "-webCertKey", filepath.Join(certDir, "privkey.pem")},
	}
	for _, args := range certCmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("set cert path: %w\n%s", err, out)
		}
	}

	return nil
}

func (e *Engine) startService() error {
	switch e.svcMgr {
	case "systemd":
		out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput()
		if err != nil {
			return fmt.Errorf("daemon-reload: %s", strings.TrimSpace(string(out)))
		}
		out, err = exec.Command("systemctl", "enable", "l-ui").CombinedOutput()
		if err != nil {
			return fmt.Errorf("enable: %s", strings.TrimSpace(string(out)))
		}
		out, err = exec.Command("systemctl", "start", "l-ui").CombinedOutput()
		if err != nil {
			return fmt.Errorf("start: %s", strings.TrimSpace(string(out)))
		}
	case "openrc":
		exec.Command("rc-update", "add", "l-ui").Run()
		exec.Command("rc-service", "l-ui", "start").Run()
	}
	return nil
}

func (e *Engine) healthCheck() error {
	return e.healthCheckWithTimeout(30)
}

func (e *Engine) healthCheckWithTimeout(maxSec int) error {
	port := e.cfg.Port
	if port == "" {
		port = "2053"
	}
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Try HTTPS first (panel may have TLS), fall back to HTTP after a few tries
	schemes := []string{"https", "http"}
	for _, scheme := range schemes {
		url := fmt.Sprintf("%s://127.0.0.1:%s/healthz", scheme, port)
		limit := maxSec
		if scheme == "https" {
			limit = 3 // give HTTPS a few tries, then fall back to HTTP
		}
		for i := 0; i < limit; i++ {
			resp, err := client.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(1 * time.Second)
		}
	}
	journal, _ := exec.Command("journalctl", "-u", "l-ui", "--no-pager", "-n", "30").CombinedOutput()
	return fmt.Errorf("service not ready after %ds\n%s", maxSec, string(journal))
}

func (e *Engine) result() *InstallResult {
	host := ""
	switch e.cfg.SSLType {
	case sslDomain:
		host = e.cfg.Domain
	case sslIP:
		host = e.cfg.IP
	}
	if host == "" {
		host = fetchPublicIP()
	}
	if host == "" {
		if ips := fetchLocalIPs(); len(ips) > 0 {
			host = ips[0]
		}
	}
	if host == "" {
		host = "localhost"
	}
	scheme := "http"
	if e.cfg.SSLType != sslNone {
		scheme = "https"
	}
	path := e.cfg.BasePath
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return &InstallResult{
		AccessURL: fmt.Sprintf("%s://%s:%s%s", scheme, host, e.cfg.Port, path),
		Username:  e.cfg.Username,
		Password:  e.cfg.Password,
		ConfigDir: e.destDir,
	}
}
