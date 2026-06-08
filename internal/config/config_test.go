package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	v := GetVersion()
	if v == "" {
		t.Fatal("version should not be empty (embedded from version file)")
	}
}

func TestGetName(t *testing.T) {
	n := GetName()
	if n == "" {
		t.Fatal("name should not be empty (embedded from name file)")
	}
}

func TestGetLogLevel_Default(t *testing.T) {
	defer os.Unsetenv("LUI_LOG_LEVEL")
	defer os.Unsetenv("LUI_DEBUG")
	os.Unsetenv("LUI_LOG_LEVEL")
	os.Unsetenv("LUI_DEBUG")

	level := GetLogLevel()
	if level != Info {
		t.Errorf("default log level = %q, want %q", level, Info)
	}
}

func TestGetLogLevel_Env(t *testing.T) {
	defer os.Unsetenv("LUI_LOG_LEVEL")
	os.Setenv("LUI_LOG_LEVEL", "debug")

	level := GetLogLevel()
	if level != Debug {
		t.Errorf("log level = %q, want %q", level, Debug)
	}
}

func TestGetLogLevel_DebugOverrides(t *testing.T) {
	defer os.Unsetenv("LUI_DEBUG")
	defer os.Unsetenv("LUI_LOG_LEVEL")
	os.Setenv("LUI_DEBUG", "true")
	os.Setenv("LUI_LOG_LEVEL", "error")

	level := GetLogLevel()
	if level != Debug {
		t.Errorf("when LUI_DEBUG=true, log level should be debug, got %q", level)
	}
}

func TestIsDebug(t *testing.T) {
	defer os.Unsetenv("LUI_DEBUG")
	os.Setenv("LUI_DEBUG", "true")
	if !IsDebug() {
		t.Error("IsDebug should be true when LUI_DEBUG=true")
	}
	os.Unsetenv("LUI_DEBUG")
	if IsDebug() {
		t.Error("IsDebug should be false when LUI_DEBUG is unset")
	}
}

func TestIsSkipHSTS(t *testing.T) {
	defer os.Unsetenv("LUI_SKIP_HSTS")
	os.Setenv("LUI_SKIP_HSTS", "true")
	if !IsSkipHSTS() {
		t.Error("IsSkipHSTS should be true")
	}
	os.Unsetenv("LUI_SKIP_HSTS")
	if IsSkipHSTS() {
		t.Error("IsSkipHSTS should be false")
	}
}

func TestGetBinFolderPath_Default(t *testing.T) {
	defer os.Unsetenv("LUI_BIN_FOLDER")
	os.Unsetenv("LUI_BIN_FOLDER")

	path := GetBinFolderPath()
	if !filepath.IsAbs(path) || !strings.HasSuffix(path, "/bin") {
		t.Errorf("default bin folder = %q, want absolute path ending in /bin", path)
	}
}

func TestGetBinFolderPath_Env(t *testing.T) {
	defer os.Unsetenv("LUI_BIN_FOLDER")
	os.Setenv("LUI_BIN_FOLDER", "/opt/l-ui/bin")

	path := GetBinFolderPath()
	if path != "/opt/l-ui/bin" {
		t.Errorf("bin folder = %q, want '/opt/l-ui/bin'", path)
	}
}

func TestGetDBFolderPath_Default(t *testing.T) {
	defer os.Unsetenv("LUI_DB_FOLDER")
	os.Unsetenv("LUI_DB_FOLDER")

	path := GetDBFolderPath()
	if path == "" {
		t.Fatal("default DB folder should not be empty")
	}
}

func TestGetDBFolderPath_Env(t *testing.T) {
	defer os.Unsetenv("LUI_DB_FOLDER")
	os.Setenv("LUI_DB_FOLDER", "/custom/db")

	path := GetDBFolderPath()
	if path != "/custom/db" {
		t.Errorf("DB folder = %q, want '/custom/db'", path)
	}
}

func TestGetDBKind_SQLite(t *testing.T) {
	defer os.Unsetenv("LUI_DB_TYPE")
	os.Unsetenv("LUI_DB_TYPE")

	kind := GetDBKind()
	if kind != "sqlite" {
		t.Errorf("default DB kind = %q, want 'sqlite'", kind)
	}
}

func TestGetDBKind_Postgres(t *testing.T) {
	defer os.Unsetenv("LUI_DB_TYPE")
	os.Setenv("LUI_DB_TYPE", "postgres")

	kind := GetDBKind()
	if kind != "postgres" {
		t.Errorf("DB kind = %q, want 'postgres'", kind)
	}
}

func TestGetDBKind_PostgresVariants(t *testing.T) {
	for _, v := range []string{"postgresql", "pg"} {
		t.Run(v, func(t *testing.T) {
			defer os.Unsetenv("LUI_DB_TYPE")
			os.Setenv("LUI_DB_TYPE", v)
			kind := GetDBKind()
			if kind != "postgres" {
				t.Errorf("GetDBKind(%q) = %q, want 'postgres'", v, kind)
			}
		})
	}
}

func TestGetDBKind_MySQL(t *testing.T) {
	defer os.Unsetenv("LUI_DB_TYPE")
	os.Setenv("LUI_DB_TYPE", "mysql")

	kind := GetDBKind()
	if kind != "mysql" {
		t.Errorf("DB kind = %q, want 'mysql'", kind)
	}
}

func TestGetDBDSN_Empty(t *testing.T) {
	defer os.Unsetenv("LUI_DB_DSN")
	os.Unsetenv("LUI_DB_DSN")

	dsn := GetDBDSN()
	if dsn != "" {
		t.Errorf("default DSN = %q, want ''", dsn)
	}
}

func TestGetDBDSN_Env(t *testing.T) {
	defer os.Unsetenv("LUI_DB_DSN")
	dsn := "postgres://user:pass@host:5432/db"
	os.Setenv("LUI_DB_DSN", dsn)

	got := GetDBDSN()
	if got != dsn {
		t.Errorf("DSN = %q, want %q", got, dsn)
	}
}

func TestGetEnvInt_Unset(t *testing.T) {
	n := GetEnvInt("LUI_NONEXISTENT_VAR_88372")
	if n != 0 {
		t.Errorf("unset var should return 0, got %d", n)
	}
}

func TestGetEnvInt_Valid(t *testing.T) {
	defer os.Unsetenv("LUI_TEST_PORT")
	os.Setenv("LUI_TEST_PORT", "8080")

	n := GetEnvInt("LUI_TEST_PORT")
	if n != 8080 {
		t.Errorf("GetEnvInt = %d, want 8080", n)
	}
}

func TestGetEnvInt_Invalid(t *testing.T) {
	defer os.Unsetenv("LUI_TEST_INVALID")
	os.Setenv("LUI_TEST_INVALID", "not-a-number")

	n := GetEnvInt("LUI_TEST_INVALID")
	if n != 0 {
		t.Errorf("invalid value should return 0, got %d", n)
	}
}

func TestGetEnvInt_Whitespace(t *testing.T) {
	defer os.Unsetenv("LUI_TEST_WS")
	os.Setenv("LUI_TEST_WS", "   ")

	n := GetEnvInt("LUI_TEST_WS")
	if n != 0 {
		t.Errorf("whitespace should return 0, got %d", n)
	}
}

func TestGetEnvFilePaths_NonWindows(t *testing.T) {
	paths := GetEnvFilePaths()
	if len(paths) == 0 {
		t.Fatal("expected non-empty env file paths on non-Windows")
	}
	if paths[0] != "/etc/default/l-ui" {
		t.Errorf("first path = %q, want '/etc/default/l-ui'", paths[0])
	}
}

func TestGetLogFolder_Default(t *testing.T) {
	defer os.Unsetenv("LUI_LOG_FOLDER")
	os.Unsetenv("LUI_LOG_FOLDER")

	path := GetLogFolder()
	if path == "" {
		t.Fatal("default log folder should not be empty")
	}
}

func TestGetLogFolder_Env(t *testing.T) {
	defer os.Unsetenv("LUI_LOG_FOLDER")
	os.Setenv("LUI_LOG_FOLDER", "/custom/log")

	path := GetLogFolder()
	if path != "/custom/log" {
		t.Errorf("log folder = %q, want '/custom/log'", path)
	}
}

func TestGetDBPath(t *testing.T) {
	defer os.Unsetenv("LUI_DB_FOLDER")
	os.Setenv("LUI_DB_FOLDER", "/tmp")

	path := GetDBPath()
	if path == "" {
		t.Fatal("DB path should not be empty")
	}
	if path != "/tmp/l-ui.db" {
		t.Errorf("DB path = %q, want '/tmp/l-ui.db'", path)
	}
}
