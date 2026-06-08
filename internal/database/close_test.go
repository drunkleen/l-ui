package database

import (
	"os"
	"testing"
)

func TestCloseDB(t *testing.T) {
	// InitDB must be called first to set the global db variable
	// We use a temporary SQLite database for testing
	tmpFile, err := os.CreateTemp("", "l-ui-test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	err = InitDB(tmpFile.Name())
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	// Close the database
	err = CloseDB()
	if err != nil {
		t.Fatalf("CloseDB returned error: %v", err)
	}

	// After CloseDB, db should be closed. Verify by checking GetDB returns nil
	// Note: The global db variable is still non-nil but the underlying connection is closed
	// We can verify by calling CloseDB again - it should not panic
	err = CloseDB()
	if err != nil {
		// Second close on already-closed DB may return an error depending on driver
		t.Logf("Second CloseDB returned: %v", err)
	}
}

func TestCloseDBWhenNil(t *testing.T) {
	// Save original db value
	originalDB := db
	db = nil
	defer func() { db = originalDB }()

	// Calling CloseDB when db is nil should return nil
	err := CloseDB()
	if err != nil {
		t.Fatalf("CloseDB when db is nil returned error: %v", err)
	}
}
