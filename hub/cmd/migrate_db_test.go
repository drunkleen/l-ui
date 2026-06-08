package cmd

import "testing"

func TestMigrateDbFlagParsing(t *testing.T) {
	migrateDsn = ""
	migrateSrc = ""

	err := migrateDbCmd.ParseFlags([]string{"--dsn", "postgres://user:pass@host/db", "--src", "/tmp/old.db"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if migrateDsn != "postgres://user:pass@host/db" {
		t.Errorf("dsn = %q, want postgres://user:pass@host/db", migrateDsn)
	}
	if migrateSrc != "/tmp/old.db" {
		t.Errorf("src = %q, want /tmp/old.db", migrateSrc)
	}
}

func TestMigrateDbSingleDashFlagParsing(t *testing.T) {
	migrateDsn = ""
	args := normalizeArgs([]string{"migrate-db", "-dsn", "postgres://user:pass@host/db"})

	err := migrateDbCmd.ParseFlags(args[1:])
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if migrateDsn != "postgres://user:pass@host/db" {
		t.Errorf("dsn = %q, want postgres://user:pass@host/db", migrateDsn)
	}
}

func TestMigrateDbCommandExists(t *testing.T) {
	if migrateDbCmd == nil {
		t.Fatal("migrateDbCmd is nil")
	}
	if migrateDbCmd.Use != "migrate-db" {
		t.Errorf("Use = %q, want migrate-db", migrateDbCmd.Use)
	}
}
