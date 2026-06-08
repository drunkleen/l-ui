package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove l-ui hub completely",
	Long: `Stops the l-ui service, removes all binaries and configuration,
and optionally deletes the database.

The database is kept by default; use --purge to remove it.`,
	RunE: runUninstall,
}

var uninstallPurge bool

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallPurge, "purge", false, "also remove the database")
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("  ⚠  This will remove l-ui completely.")
	fmt.Println("  Are you sure? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		fmt.Println("  Uninstall cancelled.")
		return nil
	}

	// Stop service
	fmt.Print("  Stopping service... ")
	exec.Command("systemctl", "stop", "l-ui").Run()
	exec.Command("systemctl", "disable", "l-ui").Run()
	fmt.Println("done")

	// Remove binary and config
	dirs := []string{
		"/usr/local/l-ui",
		"/etc/systemd/system/l-ui.service",
	}
	for _, d := range dirs {
		fmt.Printf("  Removing %s... ", d)
		os.RemoveAll(d)
		fmt.Println("done")
	}

	// Ask about database
	removeDB := uninstallPurge
	if !removeDB {
		dbPath := "/etc/l-ui/l-ui.db"
		if _, err := os.Stat(dbPath); err == nil {
			fmt.Printf("  Remove database at %s? (y/N): ", dbPath)
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(strings.ToLower(line))
			if line == "y" || line == "yes" {
				removeDB = true
			}
		}
	}
	if removeDB {
		fmt.Print("  Removing /etc/l-ui/... ")
		os.RemoveAll("/etc/l-ui")
		fmt.Println("done")
	} else {
		fmt.Println("  Database kept at /etc/l-ui/")
	}

	// Reload systemd
	exec.Command("systemctl", "daemon-reload").Run()

	fmt.Println()
	fmt.Println("  ✓  L-UI has been uninstalled.")
	if !removeDB {
		fmt.Println("  Database preserved at /etc/l-ui/ — remove manually if needed.")
	}
	return nil
}
