// Package config provides configuration management utilities for the l-ui panel,
// including version information, logging levels, database paths, and environment variable handling.
package config

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

//go:embed version
var version string

//go:embed name
var name string

// LogLevel represents the logging level for the application.
type LogLevel string

// Logging level constants
const (
	Debug   LogLevel = "debug"
	Info    LogLevel = "info"
	Notice  LogLevel = "notice"
	Warning LogLevel = "warning"
	Error   LogLevel = "error"
)

// GetVersion returns the version string of the l-ui application.
func GetVersion() string {
	return strings.TrimSpace(version)
}

// GetName returns the name of the l-ui application.
func GetName() string {
	return strings.TrimSpace(name)
}

// GetLogLevel returns the current logging level based on environment variables or defaults to Info.
func GetLogLevel() LogLevel {
	if IsDebug() {
		return Debug
	}
	logLevel := os.Getenv("LUI_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

// IsDebug returns true if debug mode is enabled via the LUI_DEBUG environment variable.
func IsDebug() bool {
	return os.Getenv("LUI_DEBUG") == "true"
}

// IsSkipHSTS returns true if skipping HSTS mode is enabled via the LUI_SKIP_HSTS environment variable.
func IsSkipHSTS() bool {
	return os.Getenv("LUI_SKIP_HSTS") == "true"
}

// GetBinFolderPath returns the path to the binary folder.
// Uses LUI_BIN_FOLDER env var, or defaults to "<binary dir>/bin".
func GetBinFolderPath() string {
	binFolderPath := os.Getenv("LUI_BIN_FOLDER")
	if binFolderPath != "" {
		return binFolderPath
	}
	return filepath.Join(getBaseDir(), "bin")
}

func getBaseDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	exeDir := filepath.Dir(exePath)
	exeDirLower := strings.ToLower(filepath.ToSlash(exeDir))
	if strings.Contains(exeDirLower, "/appdata/local/temp/") || strings.Contains(exeDirLower, "/go-build") {
		wd, err := os.Getwd()
		if err != nil {
			return "."
		}
		return wd
	}
	return exeDir
}

// GetDBFolderPath returns the path to the database folder based on environment variables or platform defaults.
func GetDBFolderPath() string {
	dbFolderPath := os.Getenv("LUI_DB_FOLDER")
	if dbFolderPath != "" {
		return dbFolderPath
	}
	if runtime.GOOS == "windows" {
		return getBaseDir()
	}
	return "/etc/l-ui"
}

// GetDBPath returns the full path to the database file.
func GetDBPath() string {
	return fmt.Sprintf("%s/%s.db", GetDBFolderPath(), GetName())
}

// GetDBKind returns the configured database backend.
func GetDBKind() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LUI_DB_TYPE")))
	switch v {
	case "postgres", "postgresql", "pg":
		return "postgres"
	case "mysql", "mariadb", "maria":
		return "mysql"
	default:
		return "sqlite"
	}
}

// GetDBDSN returns the PostgreSQL DSN from LUI_DB_DSN. Empty for sqlite.
func GetDBDSN() string {
	return strings.TrimSpace(os.Getenv("LUI_DB_DSN"))
}

// GetEnvInt returns an integer from an environment variable, or 0 if unset or invalid.
func GetEnvInt(key string) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return 0
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return n
}

// GetEnvFilePaths returns the candidate service environment file paths (the file
// systemd loads via EnvironmentFile) across the supported distro families.
func GetEnvFilePaths() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	return []string{
		"/etc/default/l-ui",
		"/etc/conf.d/l-ui",
		"/etc/sysconfig/l-ui",
	}
}

// GetLogFolder returns the path to the log folder based on environment variables or platform defaults.
func GetLogFolder() string {
	logFolderPath := os.Getenv("LUI_LOG_FOLDER")
	if logFolderPath != "" {
		return logFolderPath
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(".", "log")
	}
	return "/var/log/l-ui"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Sync()
}

func init() {
	if runtime.GOOS != "windows" {
		return
	}
	if os.Getenv("LUI_DB_FOLDER") != "" {
		return
	}
	oldDBFolder := "/etc/l-ui"
	oldDBPath := fmt.Sprintf("%s/%s.db", oldDBFolder, GetName())
	newDBFolder := GetDBFolderPath()
	newDBPath := fmt.Sprintf("%s/%s.db", newDBFolder, GetName())
	_, err := os.Stat(newDBPath)
	if err == nil {
		return // new exists
	}
	_, err = os.Stat(oldDBPath)
	if os.IsNotExist(err) {
		return // old does not exist
	}
	_ = copyFile(oldDBPath, newDBPath) // ignore error
}
