package cmd

import (
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
)

func TestSystemCommandsExist(t *testing.T) {
	cmds := []struct {
		cmd  *cobra.Command
		name string
	}{
		{geoCmd, "geo"},
		{geoUpdateCmd, "update"},
		{bbrCmd, "bbr"},
		{bbrEnableCmd, "enable"},
		{bbrDisableCmd, "disable"},
		{firewallCmd, "firewall"},
		{firewallStatusCmd, "status"},
		{firewallEnableCmd, "enable"},
		{firewallDisableCmd, "disable"},
		{firewallAllowCmd, "allow"},
		{firewallDenyCmd, "deny"},
		{iplimitCmd, "iplimit"},
		{iplimitStatusCmd, "status"},
		{iplimitEnableCmd, "enable"},
		{iplimitDisableCmd, "disable"},
		{iplimitBanCmd, "ban"},
		{iplimitUnbanCmd, "unban"},
		{postgresCmd, "postgres"},
		{postgresInstallCmd, "install"},
		{postgresStatusCmd, "status"},
		{postgresMigrateCmd, "migrate"},
		{postgresEnvCmd, "env"},
	}

	for _, tc := range cmds {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cmd == nil {
				t.Fatalf("%sCmd is nil", tc.name)
			}
		})
	}
}

func TestGeoUpdateCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	// Create a real temp directory to avoid mkdir failure
	tmpDir := t.TempDir()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		// Mock curl to succeed and create the output file
		if name == "curl" {
			// Find the -o argument and create the file
			for i, arg := range args {
				if arg == "-o" && i+1 < len(args) {
					// Create the file in the temp directory with the same filename
					f, _ := os.Create(args[i+1])
					if f != nil {
						f.WriteString("mock data")
						f.Close()
					}
				}
			}
			return exec.Command("echo")
		}
		return exec.Command("echo")
	}

	// Override geoTargetDir temporarily for testing
	oldGeoTargetDir := geoTargetDir
	geoTargetDir = tmpDir
	defer func() { geoTargetDir = oldGeoTargetDir }()

	cmd := &cobra.Command{}
	runGeoUpdate(cmd, nil)

	// Should attempt to download geoip.dat and geosite.dat
	foundGeoip := false
	foundGeosite := false
	for _, args := range capturedArgs {
		if len(args) >= 3 && args[0] == "curl" && args[len(args)-2] == "-o" {
			if args[len(args)-1] == tmpDir+"/geoip.dat" {
				foundGeoip = true
			}
			if args[len(args)-1] == tmpDir+"/geosite.dat" {
				foundGeosite = true
			}
		}
	}
	if !foundGeoip {
		t.Error("expected curl to download geoip.dat")
	}
	if !foundGeosite {
		t.Error("expected curl to download geosite.dat")
	}
}

func TestBBREnableCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		// First call is checkBBRStatus - return cubic so enable proceeds
		if name == "sysctl" && len(args) == 1 && args[0] == "net.ipv4.tcp_congestion_control" {
			return exec.Command("echo", "net.ipv4.tcp_congestion_control = cubic")
		}
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runBBREnable(cmd, nil)

	// Should set qdisc and bbr
	foundQdisc := false
	foundBBR := false
	for _, args := range capturedArgs {
		if args[0] == "sysctl" && len(args) >= 3 {
			if args[2] == "net.core.default_qdisc=fq" {
				foundQdisc = true
			}
			if args[2] == "net.ipv4.tcp_congestion_control=bbr" {
				foundBBR = true
			}
		}
	}
	if !foundQdisc {
		t.Error("expected sysctl to set net.core.default_qdisc=fq")
	}
	if !foundBBR {
		t.Error("expected sysctl to set net.ipv4.tcp_congestion_control=bbr")
	}
}

func TestBBRDisableCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		// First call is checkBBRStatus - return bbr so disable proceeds
		if name == "sysctl" && len(args) == 1 && args[0] == "net.ipv4.tcp_congestion_control" {
			return exec.Command("echo", "net.ipv4.tcp_congestion_control = bbr")
		}
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runBBRDisable(cmd, nil)

	// Should set cubic
	foundCubic := false
	for _, args := range capturedArgs {
		if args[0] == "sysctl" && len(args) >= 3 {
			if args[2] == "net.ipv4.tcp_congestion_control=cubic" {
				foundCubic = true
			}
		}
	}
	if !foundCubic {
		t.Error("expected sysctl to set net.ipv4.tcp_congestion_control=cubic")
	}
}

func TestFirewallStatusCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		if name == "which" && args[0] == "ufw" {
			return exec.Command("exit", "1")
		}
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runFirewallStatus(cmd, nil)

	if len(capturedArgs) != 1 || capturedArgs[0][0] != "which" || capturedArgs[0][1] != "ufw" {
		t.Errorf("unexpected calls: %v", capturedArgs)
	}
}

func TestFirewallStatusWithUFW(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		if name == "which" && args[0] == "ufw" {
			return exec.Command("echo", "/usr/bin/ufw")
		}
		return exec.Command("echo", "Status: active")
	}

	cmd := &cobra.Command{}
	runFirewallStatus(cmd, nil)

	foundStatus := false
	for _, args := range capturedArgs {
		if args[0] == "ufw" && args[1] == "status" {
			foundStatus = true
		}
	}
	if !foundStatus {
		t.Error("expected ufw status call")
	}
}

func TestFirewallAllowCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		if name == "which" && args[0] == "ufw" {
			return exec.Command("echo", "/usr/bin/ufw")
		}
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runFirewallAllow(cmd, []string{"8080"})

	foundAllow := false
	for _, args := range capturedArgs {
		if args[0] == "ufw" && args[1] == "allow" && args[2] == "8080" {
			foundAllow = true
		}
	}
	if !foundAllow {
		t.Error("expected ufw allow 8080 call")
	}
}

func TestFirewallDenyCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		if name == "which" && args[0] == "ufw" {
			return exec.Command("echo", "/usr/bin/ufw")
		}
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runFirewallDeny(cmd, []string{"3306"})

	foundDeny := false
	for _, args := range capturedArgs {
		if args[0] == "ufw" && args[1] == "deny" && args[2] == "3306" {
			foundDeny = true
		}
	}
	if !foundDeny {
		t.Error("expected ufw deny 3306 call")
	}
}

func TestIPLimitStatusCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		if name == "which" && args[0] == "fail2ban-client" {
			return exec.Command("echo", "/usr/bin/fail2ban-client")
		}
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runIPLimitStatus(cmd, nil)

	foundStatus := false
	for _, args := range capturedArgs {
		if args[0] == "fail2ban-client" && args[1] == "status" {
			foundStatus = true
		}
	}
	if !foundStatus {
		t.Error("expected fail2ban-client status call")
	}
}

func TestIPLimitBanCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		if name == "which" && args[0] == "fail2ban-client" {
			return exec.Command("echo", "/usr/bin/fail2ban-client")
		}
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runIPLimitBan(cmd, []string{"192.168.1.100"})

	foundBan := false
	for _, args := range capturedArgs {
		if args[0] == "fail2ban-client" && len(args) >= 5 && args[4] == "192.168.1.100" {
			foundBan = true
		}
	}
	if !foundBan {
		t.Error("expected fail2ban-client banip call")
	}
}

func TestIPLimitUnbanCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		if name == "which" && args[0] == "fail2ban-client" {
			return exec.Command("echo", "/usr/bin/fail2ban-client")
		}
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runIPLimitUnban(cmd, []string{"192.168.1.100"})

	foundUnban := false
	for _, args := range capturedArgs {
		if args[0] == "fail2ban-client" && len(args) >= 5 && args[4] == "192.168.1.100" {
			foundUnban = true
		}
	}
	if !foundUnban {
		t.Error("expected fail2ban-client unbanip call")
	}
}

func TestPostgresInstallApt(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		if name == "apt" && args[0] == "--version" {
			return exec.Command("echo", "apt 2.4.0")
		}
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runPostgresInstall(cmd, nil)

	// Should try apt
	foundApt := false
	for _, args := range capturedArgs {
		if args[0] == "sh" || args[0] == "apt" {
			foundApt = true
		}
	}
	if !foundApt {
		t.Error("expected apt install command")
	}
}

func TestPostgresStatusCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo", "localhost:5432 - accepting connections")
	}

	cmd := &cobra.Command{}
	runPostgresStatus(cmd, nil)

	if len(capturedArgs) != 1 || capturedArgs[0][0] != "pg_isready" {
		t.Errorf("expected pg_isready call, got: %v", capturedArgs)
	}
}

func TestPostgresMigrateCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runPostgresMigrate(cmd, nil)

	if len(capturedArgs) != 1 || capturedArgs[0][0] != "l-ui" || capturedArgs[0][1] != "migrate-db" {
		t.Errorf("expected l-ui migrate-db call, got: %v", capturedArgs)
	}
}

func TestPostgresEnvCommand(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	// postgres env doesn't call exec, just prints template
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo")
	}

	cmd := &cobra.Command{}
	runPostgresEnv(cmd, nil)
	// Just verify it doesn't panic
}

func TestCheckBBRStatus(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	t.Run("bbr detected", func(t *testing.T) {
		execCommand = func(name string, args ...string) *exec.Cmd {
			return exec.Command("echo", "net.ipv4.tcp_congestion_control = bbr")
		}
		status, err := checkBBRStatus()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if status != "bbr" {
			t.Errorf("status = %q, want bbr", status)
		}
	})

	t.Run("cubic detected", func(t *testing.T) {
		execCommand = func(name string, args ...string) *exec.Cmd {
			return exec.Command("echo", "net.ipv4.tcp_congestion_control = cubic")
		}
		status, err := checkBBRStatus()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if status != "cubic" {
			t.Errorf("status = %q, want cubic", status)
		}
	})
}

func TestDetectPackageManager(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	t.Run("apt detected", func(t *testing.T) {
		execCommand = func(name string, args ...string) *exec.Cmd {
			if name == "apt" && len(args) > 0 && args[0] == "--version" {
				return exec.Command("echo", "apt 2.4.0")
			}
			return exec.Command("exit", "1")
		}
		pm := detectPackageManager()
		if pm != "apt" {
			t.Errorf("pm = %q, want apt", pm)
		}
	})

	t.Run("yum detected", func(t *testing.T) {
		execCommand = func(name string, args ...string) *exec.Cmd {
			if name == "apt" {
				return exec.Command("exit", "1")
			}
			if name == "yum" && len(args) > 0 && args[0] == "--version" {
				return exec.Command("echo", "yum 4.0")
			}
			return exec.Command("exit", "1")
		}
		pm := detectPackageManager()
		if pm != "yum" {
			t.Errorf("pm = %q, want yum", pm)
		}
	})
}

func TestCheckUFWInstalled(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	t.Run("ufw installed", func(t *testing.T) {
		execCommand = func(name string, args ...string) *exec.Cmd {
			return exec.Command("echo", "/usr/bin/ufw")
		}
		if !checkUFWInstalled() {
			t.Error("expected true when ufw is installed")
		}
	})

	t.Run("ufw not installed", func(t *testing.T) {
		execCommand = func(name string, args ...string) *exec.Cmd {
			return exec.Command("exit", "1")
		}
		if checkUFWInstalled() {
			t.Error("expected false when ufw is not installed")
		}
	})
}

func TestCheckFail2banInstalled(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	t.Run("fail2ban installed", func(t *testing.T) {
		execCommand = func(name string, args ...string) *exec.Cmd {
			return exec.Command("echo", "/usr/bin/fail2ban-client")
		}
		if !checkFail2banInstalled() {
			t.Error("expected true when fail2ban is installed")
		}
	})

	t.Run("fail2ban not installed", func(t *testing.T) {
		execCommand = func(name string, args ...string) *exec.Cmd {
			return exec.Command("exit", "1")
		}
		if checkFail2banInstalled() {
			t.Error("expected false when fail2ban is not installed")
		}
	})
}

func TestGeoTargetDir(t *testing.T) {
	if geoTargetDir != "/usr/local/l-ui" {
		t.Errorf("geoTargetDir = %q, want /usr/local/l-ui", geoTargetDir)
	}
}
