package sshutil

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/drunkleen/l-ui/internal/util/common"
	"github.com/drunkleen/l-ui/internal/util/random"
	"golang.org/x/crypto/ssh"
)

func SshAddress(host string, port int) string {
	if port <= 0 {
		port = 22
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func SshAuthMethods(key, passphrase, password string) ([]ssh.AuthMethod, error) {
	methods := make([]ssh.AuthMethod, 0, 2)
	if strings.TrimSpace(key) != "" {
		var signer ssh.Signer
		var err error
		keyBytes := []byte(key)
		if strings.TrimSpace(passphrase) != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}
		if err != nil {
			return nil, fmt.Errorf("parse ssh private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if strings.TrimSpace(password) != "" {
		methods = append(methods, ssh.Password(password))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("ssh password or private key is required")
	}
	return methods, nil
}

func RunSSHCommand(client *ssh.Client, password string, useSudo bool, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	var out bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &out

	var remoteCmd string
	var stdin io.WriteCloser

	if useSudo {
		stdin, err = sess.StdinPipe()
		if err != nil {
			return "", err
		}
		remoteCmd = "sudo -S -p '' sh -s"
	} else {
		remoteCmd = cmd
	}

	if err := sess.Start(remoteCmd); err != nil {
		return out.String(), err
	}

	if useSudo {
		if password != "" {
			_, _ = io.WriteString(stdin, password+"\n")
		}
		_, _ = io.WriteString(stdin, cmd+"\n")
		_ = stdin.Close()
	}

	if err := sess.Wait(); err != nil {
		return out.String(), err
	}
	return strings.TrimSpace(out.String()), nil
}

func RemoteCommand(client *ssh.Client, cmd string) (string, error) {
	return RunSSHCommand(client, "", false, cmd)
}

func UploadRemoteFile(client *ssh.Client, password string, useSudo bool, remotePath string, data []byte, mode string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	return UploadRemoteFileSession(&SshUploadSessionAdapter{Session: sess}, password, useSudo, remotePath, data, mode)
}

type SshUploadSession interface {
	StdinPipe() (io.WriteCloser, error)
	Start(string) error
	Wait() error
	Close() error
	SetStdout(io.Writer)
	SetStderr(io.Writer)
}

type SshUploadSessionAdapter struct {
	Session *ssh.Session
}

func (s *SshUploadSessionAdapter) StdinPipe() (io.WriteCloser, error) { return s.Session.StdinPipe() }
func (s *SshUploadSessionAdapter) Start(cmd string) error             { return s.Session.Start(cmd) }
func (s *SshUploadSessionAdapter) Wait() error                        { return s.Session.Wait() }
func (s *SshUploadSessionAdapter) Close() error                       { return s.Session.Close() }
func (s *SshUploadSessionAdapter) SetStdout(w io.Writer)              { s.Session.Stdout = w }
func (s *SshUploadSessionAdapter) SetStderr(w io.Writer)              { s.Session.Stderr = w }

func UploadRemoteFileSession(sess SshUploadSession, password string, useSudo bool, remotePath string, data []byte, mode string) (string, error) {
	var out bytes.Buffer
	sess.SetStdout(&out)
	sess.SetStderr(&out)

	innerCmd := fmt.Sprintf("cat > %s && chmod %s %s", common.ShellQuote(remotePath), mode, common.ShellQuote(remotePath))
	remoteCmd := innerCmd
	var stdin io.WriteCloser
	if useSudo {
		remoteCmd = "sudo -S -p '' sh -lc " + common.ShellQuote(innerCmd)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return out.String(), err
	}

	if err := sess.Start(remoteCmd); err != nil {
		return out.String(), err
	}

	if useSudo {
		if password != "" {
			if _, err := io.WriteString(stdin, password+"\n"); err != nil {
				_ = stdin.Close()
				return out.String(), err
			}
		}
		if _, err := stdin.Write(data); err != nil {
			_ = stdin.Close()
			return out.String(), err
		}
		_ = stdin.Close()
	} else {
		if _, err := stdin.Write(data); err != nil {
			_ = stdin.Close()
			return out.String(), err
		}
		_ = stdin.Close()
	}

	if err := sess.Wait(); err != nil {
		return out.String(), err
	}
	return strings.TrimSpace(out.String()), nil
}

func InstallServiceFallback(conn *ssh.Client, password string, useSudo bool) (string, error) {
	releaseOut, err := RunSSHCommand(conn, password, useSudo, `. /etc/os-release >/dev/null 2>&1; printf '%s' "${ID:-}"`)
	if err != nil {
		return releaseOut, err
	}
	release := strings.TrimSpace(releaseOut)
	serviceData, serviceName, err := LocalServiceUnitForRelease(release)
	if err != nil {
		return "", err
	}
	remoteTmp := "/tmp/" + serviceName
	out, err := UploadRemoteFile(conn, password, useSudo, remoteTmp, serviceData, "0644")
	if err != nil {
		return out, err
	}
	installCmd := fmt.Sprintf(`set -e
cp -f %s /etc/systemd/system/l-ui.service
chown root:root /etc/systemd/system/l-ui.service
chmod 644 /etc/systemd/system/l-ui.service
rm -f %s
`, remoteTmp, remoteTmp)
	copyOut, err := RunSSHCommand(conn, password, useSudo, installCmd)
	if err != nil {
		return out + "\n" + copyOut, err
	}
	if strings.TrimSpace(copyOut) != "" {
		return out + "\n" + copyOut, nil
	}
	return out, nil
}

func ShouldInstallServiceFallback(output string, err error) bool {
	msg := output
	if err != nil {
		msg += "\n" + err.Error()
	}
	msg = strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(msg, "missing l-ui.service in bundle")
}

func LocalServiceUnitForRelease(release string) ([]byte, string, error) {
	release = strings.ToLower(strings.TrimSpace(release))
	servicePath := "l-ui.service.rhel"
	switch release {
	case "ubuntu", "debian", "armbian":
		servicePath = "l-ui.service.debian"
	case "arch", "manjaro", "parch":
		servicePath = "l-ui.service.arch"
	}
	repoRoot, err := FindRepoRoot()
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, servicePath))
	if err != nil {
		return nil, "", err
	}
	return data, filepath.Base(servicePath), nil
}

func FindRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", fmt.Errorf("repository root not found from %s", wd)
		}
		wd = parent
	}
}

func BootstrapAgentPort(port int) int {
	if port > 0 {
		return port
	}
	return 2001 + random.Num(65535-2000)
}
