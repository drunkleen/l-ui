package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/drunkleen/l-ui/internal/bundle"
	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/nodeauth"
	"github.com/drunkleen/l-ui/internal/sshutil"
	archutil "github.com/drunkleen/l-ui/internal/util/arch"
	"github.com/drunkleen/l-ui/internal/util/common"
	"github.com/drunkleen/l-ui/internal/util/random"
	"github.com/drunkleen/l-ui/internal/util/retry"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type BootstrapStep struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
}

type NodeBootstrapRequest struct {
	Name             string `json:"name" form:"name" validate:"required"`
	Address          string `json:"address" form:"address" validate:"required"`
	SSHUser          string `json:"sshUser" form:"sshUser" validate:"required"`
	SSHPassword      string `json:"sshPassword" form:"sshPassword"`
	SSHPrivateKey    string `json:"sshPrivateKey,omitempty" form:"sshPrivateKey"`
	SSHKeyPassphrase string `json:"sshKeyPassphrase,omitempty" form:"sshKeyPassphrase"`
	SSHPort          int    `json:"sshPort" form:"sshPort" validate:"omitempty,gte=1,lte=65535"`
	AgentPort        int    `json:"agentPort" form:"agentPort" validate:"omitempty,gte=1,lte=65535"`
	UseTLS           bool   `json:"useTLS" form:"useTLS"`
	Domain           string `json:"domain,omitempty" form:"domain"`
	AcmeEmail        string `json:"acmeEmail,omitempty" form:"acmeEmail"`
	DNSProvider      string `json:"dnsProvider,omitempty" form:"dnsProvider"`
	BootstrapBase    string `json:"bootstrapBase,omitempty" form:"bootstrapBase"`
}

type NodeBootstrapResult struct {
	Node  *model.Node     `json:"node"`
	Steps []BootstrapStep `json:"steps"`
}

type NodeBootstrapJob struct {
	ID    string          `json:"id"`
	State string          `json:"state"`
	Step  string          `json:"step,omitempty"`
	Error string          `json:"error,omitempty"`
	Node  *model.Node     `json:"node,omitempty"`
	Steps []BootstrapStep `json:"steps,omitempty"`
}

const (
	bootstrapStateQueued  = "queued"
	bootstrapStateRunning = "running"
	bootstrapStateDone    = "done"
	bootstrapStateFailed  = "failed"
)

func (s *NodeService) bootstrapJobStore() map[string]*NodeBootstrapJob {
	if s.bootstrapJobs == nil {
		s.bootstrapJobs = map[string]*NodeBootstrapJob{}
	}
	return s.bootstrapJobs
}

func (s *NodeService) putBootstrapJob(job *NodeBootstrapJob) {
	s.bootstrapMu.Lock()
	defer s.bootstrapMu.Unlock()
	s.bootstrapJobStore()[job.ID] = job
}

func (s *NodeService) getBootstrapJob(id string) (*NodeBootstrapJob, bool) {
	s.bootstrapMu.Lock()
	defer s.bootstrapMu.Unlock()
	job, ok := s.bootstrapJobStore()[id]
	if !ok || job == nil {
		return nil, false
	}
	copyJob := *job
	if len(job.Steps) > 0 {
		copyJob.Steps = append([]BootstrapStep(nil), job.Steps...)
	}
	return &copyJob, true
}

func (s *NodeService) updateBootstrapJob(id string, apply func(job *NodeBootstrapJob)) {
	s.bootstrapMu.Lock()
	defer s.bootstrapMu.Unlock()
	job, ok := s.bootstrapJobStore()[id]
	if !ok || job == nil {
		return
	}
	apply(job)
}

func (s *NodeService) BootstrapJob(id string) (*NodeBootstrapJob, bool) {
	return s.getBootstrapJob(id)
}

func (s *NodeService) StartBootstrap(ctx context.Context, req NodeBootstrapRequest) (*NodeBootstrapJob, error) {
	if req.Address == "" {
		return nil, nodeConfigErr("node address is required")
	}
	job := &NodeBootstrapJob{ID: random.Seq(24), State: bootstrapStateQueued}
	s.putBootstrapJob(job)
	go func() {
		result, err := s.bootstrapFlow(context.Background(), req, func(step BootstrapStep) {
			s.updateBootstrapJob(job.ID, func(j *NodeBootstrapJob) {
				j.State = bootstrapStateRunning
				j.Step = step.Name
				j.Steps = append(j.Steps, step)
			})
		})
		if err != nil {
			s.updateBootstrapJob(job.ID, func(j *NodeBootstrapJob) {
				j.State = bootstrapStateFailed
				j.Error = err.Error()
				if result != nil {
					j.Node = result.Node
					j.Steps = append([]BootstrapStep(nil), result.Steps...)
				}
			})
			return
		}
		s.updateBootstrapJob(job.ID, func(j *NodeBootstrapJob) {
			j.State = bootstrapStateDone
			j.Node = result.Node
			j.Steps = append([]BootstrapStep(nil), result.Steps...)
		})
	}()
	return job, nil
}

func (s *NodeService) buildBootstrapNode(req NodeBootstrapRequest) (model.Node, error) {
	if req.Name == "" {
		return model.Node{}, nodeConfigErr("node name is required")
	}
	if req.Address == "" {
		return model.Node{}, nodeConfigErr("node address is required")
	}
	req.AgentPort = sshutil.BootstrapAgentPort(req.AgentPort)
	node := model.Node{
		Name:                req.Name,
		Address:             req.Address,
		Scheme:              "http",
		Port:                req.AgentPort,
		BasePath:            "/",
		Enable:              true,
		AllowPrivateAddress: true,
		TlsVerifyMode:       "verify",
		ApiToken:            random.Seq(48),
	}
	if req.BootstrapBase != "" {
		node.BasePath = common.NormalizeBasePath(req.BootstrapBase)
	}
	if req.UseTLS {
		if strings.TrimSpace(req.Domain) == "" {
			return model.Node{}, nodeConfigErr("domain is required when TLS bootstrap is enabled")
		}
		node.Scheme = "https"
		node.Address = strings.TrimSpace(req.Domain)
		node.Port = 443
		node.AllowPrivateAddress = false
	}
	if err := s.normalize(&node); err != nil {
		return model.Node{}, err
	}
	node.Scheme = strings.ToLower(strings.TrimSpace(node.Scheme))
	if node.Scheme == "" {
		node.Scheme = "http"
	}
	if !req.UseTLS {
		node.Port = req.AgentPort
	}
	if req.BootstrapBase == "" {
		node.BasePath = "/"
	}
	node.ApiToken = strings.TrimSpace(node.ApiToken)
	return node, nil
}

func (s *NodeService) prepareBootstrapNodeIdentity(node *model.Node) error {
	db := database.GetDB()
	if db == nil {
		return nil
	}
	var existing model.Node
	if err := db.Where("name = ?", node.Name).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	node.Id = existing.Id
	node.ApiToken = existing.ApiToken
	return nil
}

func (s *NodeService) persistBootstrapNode(node *model.Node) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	if node.Id == 0 {
		return s.Create(node)
	}
	if err := s.normalize(node); err != nil {
		return err
	}
	updates := map[string]any{
		"name":                  node.Name,
		"scheme":                node.Scheme,
		"address":               node.Address,
		"port":                  node.Port,
		"base_path":             node.BasePath,
		"api_token":             node.ApiToken,
		"enable":                node.Enable,
		"allow_private_address": node.AllowPrivateAddress,
		"tls_verify_mode":       node.TlsVerifyMode,
		"pinned_cert_sha256":    node.PinnedCertSha256,
		"bundle_sha256":         node.BundleSHA256,
	}
	return db.Model(&model.Node{}).Where("id = ?", node.Id).Updates(updates).Error
}

func bootstrapCaddyfile(domain, email string, port int, tlsDirective string) string {
	var b strings.Builder
	if strings.TrimSpace(email) != "" {
		b.WriteString("{\n")
		b.WriteString("    email ")
		b.WriteString(strings.TrimSpace(email))
		b.WriteString("\n}\n\n")
	}
	b.WriteString(strings.TrimSpace(domain))
	b.WriteString(" {\n    reverse_proxy 127.0.0.1:")
	b.WriteString(strconv.Itoa(port))
	if strings.TrimSpace(tlsDirective) != "" {
		b.WriteString("\n    tls {")
		b.WriteString("\n        ")
		b.WriteString(strings.TrimSpace(tlsDirective))
		b.WriteString("\n    }")
	}
	b.WriteString("\n}\n")
	return b.String()
}

func bootstrapTLSDirective(providerName, domain string) (string, error) {
	if strings.TrimSpace(providerName) == "" {
		return "", nil
	}
	provider := getDNSProvider(providerName)
	if provider == nil {
		return "", nodeConfigErr("dns provider not registered: " + providerName)
	}
	return provider.TLSDirective(domain)
}

func (s *NodeService) Bootstrap(ctx context.Context, req NodeBootstrapRequest) (*NodeBootstrapResult, error) {
	return s.bootstrapFlow(ctx, req, nil)
}

// bootstrapCleanup runs rollback commands on the node based on how far
// bootstrapFlow progressed. stage 0 = tarball only, 1 = +extracted files,
// 2+ = full rollback (stop service, restore previous).
func (s *NodeService) bootstrapCleanup(conn *ssh.Client, req NodeBootstrapRequest, useSudo bool, stage int) (string, error) {
	var cmd string
	switch {
	case stage >= 2:
		cmd = `set -e
systemctl stop l-ui-agent || true
rm -rf /usr/local/l-ui-agent
rm -f /etc/l-ui/l-ui.db
rm -f /tmp/l-ui-agent.tar.gz
if [ -d /usr/local/l-ui-agent.previous ]; then
  mv /usr/local/l-ui-agent.previous /usr/local/l-ui-agent
fi
systemctl daemon-reload
systemctl enable --now l-ui-agent || true
`
	case stage >= 1:
		cmd = `set -e
rm -f /tmp/l-ui-agent.tar.gz
rm -rf /usr/local/l-ui-agent
`
	default:
		cmd = `rm -f /tmp/l-ui-agent.tar.gz`
	}
	return sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, cmd)
}

func (s *NodeService) bootstrapFlow(ctx context.Context, req NodeBootstrapRequest, report func(BootstrapStep)) (*NodeBootstrapResult, error) {
	if req.Address == "" {
		return nil, nodeConfigErr("node address is required")
	}
	req.AgentPort = sshutil.BootstrapAgentPort(req.AgentPort)
	if req.SSHPort <= 0 {
		req.SSHPort = 22
	}
	node, err := s.buildBootstrapNode(req)
	if err != nil {
		return nil, err
	}

	addr := sshutil.SshAddress(req.Address, req.SSHPort)
	authMethods, err := sshutil.SshAuthMethods(req.SSHPrivateKey, req.SSHKeyPassphrase, req.SSHPassword)
	if err != nil {
		return nil, err
	}
	sshCfg := &ssh.ClientConfig{
		User:            req.SSHUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         20 * time.Second,
	}
	var conn *ssh.Client
	if err := retry.Do(ctx, retry.Config{
		MaxAttempts:    3,
		InitialBackoff: 2 * time.Second,
		MaxBackoff:     10 * time.Second,
		JitterFactor:   0.2,
	}, func(ctx context.Context) error {
		var dialErr error
		conn, dialErr = ssh.Dial("tcp", addr, sshCfg)
		return dialErr
	}); err != nil {
		return nil, nodeServiceErr(fmt.Sprintf("ssh connect to %s after retries: %v", addr, err))
	}
	defer conn.Close()

	uidOut, err := sshutil.RemoteCommand(conn, "id -u")
	if err != nil {
		return nil, nodeServiceErr(fmt.Sprintf("detect remote privilege: %v", err))
	}
	useSudo := strings.TrimSpace(uidOut) != "0"
	if useSudo {
		if _, err := sshutil.RunSSHCommand(conn, req.SSHPassword, true, "sudo -v"); err != nil {
			return nil, nodeServiceErr(fmt.Sprintf("remote user needs sudo access: %v", err))
		}
	}
	if _, err := sshutil.RemoteCommand(conn, "command -v systemctl >/dev/null"); err != nil {
		return nil, nodeServiceErr("systemd/systemctl is required on the remote host")
	}

	steps := make([]BootstrapStep, 0, 8)
	addStep := func(name string, ok bool, output string) {
		step := BootstrapStep{Name: name, OK: ok, Output: strings.TrimSpace(output)}
		steps = append(steps, step)
		if report != nil {
			report(step)
		}
	}

	stepCmds := []struct {
		name string
		cmd  string
	}{
		{"detect-arch", "uname -m"},
		{"prepare-dirs", "mkdir -p /usr/local/l-ui-agent/bin /etc/l-ui /var/log/l-ui"},
	}

	var arch string
	for _, step := range stepCmds {
		var out string
		var err error
		if step.name == "detect-arch" {
			out, err = sshutil.RemoteCommand(conn, step.cmd)
		} else {
			out, err = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, step.cmd)
		}
		if err != nil {
			addStep(step.name, false, out+"\n"+err.Error())
			return &NodeBootstrapResult{Node: &node, Steps: steps}, err
		}
		addStep(step.name, true, out)
		switch step.name {
		case "detect-arch":
			if strings.TrimSpace(out) == "" {
				out, err = sshutil.RemoteCommand(conn, "/bin/uname -m")
				if err != nil {
					addStep(step.name, false, out+"\n"+err.Error())
					return &NodeBootstrapResult{Node: &node, Steps: steps}, err
				}
				addStep("detect-arch-retry", true, out)
			}
			arch, err = archutil.SSHBootstrapArch(out)
			if err != nil {
				addStep("map-arch", false, fmt.Sprintf("raw SSH output: %q — %v", out, err))
				return &NodeBootstrapResult{Node: &node, Steps: steps}, err
			}
			addStep("map-arch", true, arch)
		}
	}

	version, err := bundle.BundleVersion()
	if err != nil {
		addStep("resolve-version", false, err.Error())
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	addStep("resolve-version", true, version)

	if err := s.prepareBootstrapNodeIdentity(&node); err != nil {
		addStep("prepare-node", false, err.Error())
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}

	// ── curl availability check ──────────────────────────────────────
	if _, err := sshutil.RemoteCommand(conn, "command -v curl >/dev/null 2>&1"); err != nil {
		addStep("check-curl", false, "curl is required on the remote server for downloading the agent bundle")
		return &NodeBootstrapResult{Node: &node, Steps: steps}, fmt.Errorf("curl is required on the remote server")
	}
	addStep("check-curl", true, "curl found")

	// ── download agent bundle via curl on the node ────────────────────
	releaseURL := bundle.BundleReleaseURL(version, arch)
	dlPath := "/tmp/l-ui-agent.tar.gz"
	downloadCmd := fmt.Sprintf("curl -fL -o %s %s", dlPath, releaseURL)

	var out string
	out, err = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, downloadCmd)
	if err != nil {
		if dlErr := retry.Do(ctx, retry.Config{
			MaxAttempts:    3,
			InitialBackoff: 2 * time.Second,
			MaxBackoff:     10 * time.Second,
			JitterFactor:   0.2,
		}, func(ctx context.Context) error {
			var retryErr error
			out, retryErr = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, downloadCmd)
			return retryErr
		}); dlErr != nil {
			addStep("download-bundle", false, out+"\n"+dlErr.Error())
			_, _ = s.bootstrapCleanup(conn, req, useSudo, 1)
			return &NodeBootstrapResult{Node: &node, Steps: steps}, dlErr
		}
	}
	addStep("download-bundle", true, out)

	// ── stages completed (used for rollback) ─────────────────────────
	completed := 0

	extractCmd := `set -e
mkdir -p /usr/local /etc/default /var/log/l-ui
rm -f /etc/l-ui/l-ui.db
if [ -d /usr/local/l-ui-agent ]; then
  rm -rf /usr/local/l-ui-agent.previous
  mv /usr/local/l-ui-agent /usr/local/l-ui-agent.previous
fi
tar -xzf /tmp/l-ui-agent.tar.gz -C /usr/local
# Backward compat: old-format bundles extract as l-ui/ instead of l-ui-agent/
if [ -d /usr/local/l-ui ] && [ ! -d /usr/local/l-ui-agent ]; then
  mv /usr/local/l-ui /usr/local/l-ui-agent
fi
if [ -d /usr/local/l-ui.previous ] && [ ! -d /usr/local/l-ui-agent.previous ]; then
  mv /usr/local/l-ui.previous /usr/local/l-ui-agent.previous
fi
`
	out, err = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, extractCmd)
	if err != nil {
		addStep("install-bundle", false, out+"\n"+err.Error())
		_, _ = s.bootstrapCleanup(conn, req, useSudo, 1)
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	completed = 1

	envCmd := fmt.Sprintf(`set -e
mkdir -p /etc/default
cat >/etc/default/l-ui-agent <<'EOF'
LUI_DB_FOLDER=/etc/l-ui
LUI_MAIN_FOLDER=/usr/local/l-ui-agent
LUI_SERVICE=/etc/systemd/system
LUI_BOOTSTRAP_API_TOKEN=%s
LUI_WEB_PORT=%d
EOF`, node.ApiToken, req.AgentPort)
	envCmd += "\nchmod 600 /etc/default/l-ui-agent\n"
	out, err = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, envCmd)
	if err != nil {
		addStep("write-env", false, out+"\n"+err.Error())
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	addStep("write-env", true, out)

	serviceCmd := fmt.Sprintf(`set -e
release="$(. /etc/os-release >/dev/null 2>&1; echo "${ID:-}")"
if [ -f /usr/local/l-ui-agent/l-ui-agent.service ]; then
  cp -f /usr/local/l-ui-agent/l-ui-agent.service /etc/systemd/system/l-ui-agent.service
elif [ -f /usr/local/l-ui-agent/l-ui-agent.service.debian ]; then
  case "$release" in
    ubuntu|debian|armbian) cp -f /usr/local/l-ui-agent/l-ui-agent.service.debian /etc/systemd/system/l-ui-agent.service ;;
    arch|manjaro|parch) cp -f /usr/local/l-ui-agent/l-ui-agent.service.arch /etc/systemd/system/l-ui-agent.service ;;
    *) cp -f /usr/local/l-ui-agent/l-ui-agent.service.rhel /etc/systemd/system/l-ui-agent.service ;;
  esac
elif [ -f /usr/local/l-ui-agent/l-ui-agent.service.arch ]; then
  case "$release" in
    arch|manjaro|parch) cp -f /usr/local/l-ui-agent/l-ui-agent.service.arch /etc/systemd/system/l-ui-agent.service ;;
    ubuntu|debian|armbian) cp -f /usr/local/l-ui-agent/l-ui-agent.service.debian /etc/systemd/system/l-ui-agent.service ;;
    *) cp -f /usr/local/l-ui-agent/l-ui-agent.service.rhel /etc/systemd/system/l-ui-agent.service ;;
  esac
elif [ -f /usr/local/l-ui-agent/l-ui-agent.service.rhel ]; then
  case "$release" in
    ubuntu|debian|armbian) cp -f /usr/local/l-ui-agent/l-ui-agent.service.rhel /etc/systemd/system/l-ui-agent.service ;;
    arch|manjaro|parch) cp -f /usr/local/l-ui-agent/l-ui-agent.service.arch /etc/systemd/system/l-ui-agent.service ;;
    *) cp -f /usr/local/l-ui-agent/l-ui-agent.service.rhel /etc/systemd/system/l-ui-agent.service ;;
  esac
else
  # Backward compat: old-format bundles have l-ui.service* instead of l-ui-agent.service*
  if [ -f /usr/local/l-ui-agent/l-ui.service ]; then
    cp -f /usr/local/l-ui-agent/l-ui.service /etc/systemd/system/l-ui-agent.service
    # Strip 'run' subcommand from old hub service files
    sed -i 's|ExecStart=/usr/local/l-ui-agent/l-ui run|ExecStart=/usr/local/l-ui-agent/l-ui-agent|' /etc/systemd/system/l-ui-agent.service
  elif [ -f /usr/local/l-ui-agent/l-ui.service.debian ]; then
    case "$release" in
      ubuntu|debian|armbian) cp -f /usr/local/l-ui-agent/l-ui.service.debian /etc/systemd/system/l-ui-agent.service ;;
      arch|manjaro|parch) cp -f /usr/local/l-ui-agent/l-ui.service.arch /etc/systemd/system/l-ui-agent.service ;;
      *) cp -f /usr/local/l-ui-agent/l-ui.service.rhel /etc/systemd/system/l-ui-agent.service ;;
    esac
    sed -i 's|ExecStart=/usr/local/l-ui-agent/l-ui run|ExecStart=/usr/local/l-ui-agent/l-ui-agent|' /etc/systemd/system/l-ui-agent.service
  else
    echo 'missing l-ui-agent.service in bundle'; exit 1
  fi
fi
chown root:root /etc/systemd/system/l-ui-agent.service
chmod 644 /etc/systemd/system/l-ui-agent.service
`)

	out, err = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, serviceCmd)
	if err != nil {
		if sshutil.ShouldInstallServiceFallback(out, err) {
			if fbOut, fbErr := sshutil.InstallServiceFallback(conn, req.SSHPassword, useSudo); fbErr == nil {
				addStep("install-service", true, fbOut)
				addStep("install-service-fallback", true, fbOut)
			} else {
				addStep("install-service", false, out+"\n"+err.Error())
				addStep("install-service-fallback", false, fbOut+"\n"+fbErr.Error())
				return &NodeBootstrapResult{Node: &node, Steps: steps}, err
			}
		} else {
			addStep("install-service", false, out+"\n"+err.Error())
			return &NodeBootstrapResult{Node: &node, Steps: steps}, err
		}
	} else {
		addStep("install-service", true, out)
	}

	binaryCheckCmd := `set -e
if [ ! -f /usr/local/l-ui-agent/l-ui-agent ]; then
  # Backward compat: old-format bundles have l-ui instead of l-ui-agent
  if [ -f /usr/local/l-ui-agent/l-ui ]; then
    ln -sf l-ui /usr/local/l-ui-agent/l-ui-agent
  else
    echo 'missing l-ui-agent executable in bundle'; exit 1
  fi
fi
chmod 755 /usr/local/l-ui-agent/l-ui-agent
`
	out, err = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, binaryCheckCmd)
	if err != nil {
		addStep("verify-bundle", false, out+"\n"+err.Error())
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	addStep("verify-bundle", true, out)

	svcStartSteps := []struct {
		name string
		cmd  string
	}{
		{"daemon-reload", "systemctl daemon-reload"},
		{"enable-service", "systemctl enable l-ui-agent"},
		{"restart-service", `set -e; systemctl restart l-ui-agent 2>&1; sleep 1; journalctl -u l-ui-agent --no-pager -n 30 2>&1`},
	}
	for _, st := range svcStartSteps {
		out, err = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, st.cmd)
		if err != nil {
			addStep(st.name, false, out+"\n"+err.Error())
			// Check what binary is at the path and try running it directly.
			diagOut, _ := sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo,
				`ls -la /usr/local/l-ui-agent/l-ui-agent; echo "---"; file /usr/local/l-ui-agent/l-ui-agent 2>&1; echo "---run---"; timeout 3 /usr/local/l-ui-agent/l-ui-agent 2>&1 || true`)
			addStep("service-diag", false, diagOut)
			if rbOut, rbErr := s.bootstrapCleanup(conn, req, useSudo, completed); rbErr == nil {
				addStep("rollback", true, rbOut)
			} else {
				addStep("rollback", false, rbOut+"\n"+rbErr.Error())
			}
			return &NodeBootstrapResult{Node: &node, Steps: steps}, err
		}
		addStep(st.name, true, out)
	}

	// ── Open UFW ports (even if disabled — user may enable later) ────
	sslPorts := ""
	if req.UseTLS {
		sslPorts = fmt.Sprintf(`
ufw allow 80
echo "ufw: port 80 allowed (for SSL)"
ufw allow 443
echo "ufw: port 443 allowed (for Caddy HTTPS)"`)
	}
	ufwCmd := fmt.Sprintf(`set -e
if command -v ufw >/dev/null 2>&1; then
  ufw allow %d
  echo "ufw: port %d allowed"%s
else
  echo "ufw not installed, skipping"
fi
`, req.AgentPort, req.AgentPort, sslPorts)
	out, err = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, ufwCmd)
	if err != nil {
		addStep("open-ufw", true, "[non-fatal] "+out+"\n"+err.Error())
	} else {
		addStep("open-ufw", true, out)
	}

	if req.UseTLS {
		caddyArch, err := archutil.CaddyAssetArch(arch)
		if err != nil {
			addStep("map-caddy-arch", false, err.Error())
			return &NodeBootstrapResult{Node: &node, Steps: steps}, err
		}
		tlsDirective, err := bootstrapTLSDirective(req.DNSProvider, req.Domain)
		if err != nil {
			addStep("prepare-dns-provider", false, err.Error())
			return &NodeBootstrapResult{Node: &node, Steps: steps}, err
		}
		installCaddyCmd := fmt.Sprintf(`set -e
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
asset=$(curl -fsSL https://api.github.com/repos/caddyserver/caddy/releases/latest | grep -o '"browser_download_url"[[:space:]]*:[[:space:]]*"[^"]*linux_%s[^"]*tar.gz"' | head -n1 | cut -d'"' -f4)
[ -n "$asset" ] || { echo 'caddy release asset not found'; exit 1; }
curl -fL -o "$tmpdir/caddy.tgz" "$asset"
tar -xzf "$tmpdir/caddy.tgz" -C "$tmpdir"
cp "$tmpdir/caddy" /usr/local/bin/caddy
chmod +x /usr/local/bin/caddy
mkdir -p /etc/caddy /var/lib/caddy
cat >/etc/caddy/Caddyfile <<'EOF'
%s
EOF
cat >/etc/systemd/system/caddy.service <<'EOF'
[Unit]
Description=Caddy reverse proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
ExecReload=/usr/local/bin/caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile --force
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now caddy
systemctl is-active --quiet caddy
`, caddyArch, bootstrapCaddyfile(req.Domain, req.AcmeEmail, req.AgentPort, tlsDirective))
		out, err = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, installCaddyCmd)
		if err != nil {
			addStep("install-caddy", false, out+"\n"+err.Error())
			return &NodeBootstrapResult{Node: &node, Steps: steps}, err
		}
		addStep("install-caddy", true, out)

		// ── Verify Caddy is serving HTTPS ─────────────────────────
		caddyCheckCmd := fmt.Sprintf(`set -e
for i in $(seq 1 12); do
  if curl -fsSk --resolve "%s:443:127.0.0.1" \
    "https://%s/api/v1/status" >/dev/null 2>&1; then
    echo "caddy: HTTPS reachable, agent responding"
    exit 0
  fi
  sleep 5
done
echo "caddy: agent not reachable through HTTPS after 60s"
exit 1
`, strings.TrimSpace(req.Domain), strings.TrimSpace(req.Domain))
		chkOut, chkErr := sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, caddyCheckCmd)
		if chkErr != nil {
			addStep("verify-caddy", false, chkOut+"\n"+chkErr.Error())
		} else {
			addStep("verify-caddy", true, chkOut)
		}

		node.Scheme = "https"
		node.Address = strings.TrimSpace(req.Domain)
		node.Port = 443
		node.AllowPrivateAddress = false
	}

	timestamp := time.Now().Unix()
	nonce := random.Seq(24)
	bodyDigest := nodeauth.BodyDigest(nil)
	signature := nodeauth.Sign(node.ApiToken, http.MethodGet, "/api/v1/status", nil, timestamp, nonce)
	verifyCmd := fmt.Sprintf(`for i in $(seq 1 30); do
  for _scheme in https http; do
    if curl -fsSk \
      -H "Authorization: Bearer %[1]s" \
      -H "X-LUI-Timestamp: %[2]d" \
      -H "X-LUI-Nonce: %[3]s" \
      -H "X-LUI-Signature: %[4]s" \
      -H "X-LUI-Body-SHA256: %[5]s" \
      "${_scheme}://127.0.0.1:%[6]d/api/v1/status" >/tmp/lui-bootstrap-status.json 2>/dev/null; then
      cat /tmp/lui-bootstrap-status.json
      exit 0
    fi
  done
  sleep $((2 + RANDOM %% 3))
done
exit 1`, node.ApiToken, timestamp, nonce, signature, bodyDigest, req.AgentPort)

	out, err = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, verifyCmd)
	if err != nil {
		addStep("verify-agent", false, out+"\n"+err.Error())
		if rbOut, rbErr := s.bootstrapCleanup(conn, req, useSudo, completed); rbErr == nil {
			addStep("rollback", true, rbOut)
		} else {
			addStep("rollback", false, rbOut+"\n"+rbErr.Error())
		}
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	addStep("verify-agent", true, out)
	_, _ = sshutil.RunSSHCommand(conn, req.SSHPassword, useSudo, `rm -rf /usr/local/l-ui-agent.previous`)
	node.BundleSHA256 = version

	if err := s.persistBootstrapNode(&node); err != nil {
		return &NodeBootstrapResult{Node: &node, Steps: steps}, err
	}
	return &NodeBootstrapResult{Node: &node, Steps: steps}, nil
}
