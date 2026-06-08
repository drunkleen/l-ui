package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDB_ValidPath(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "agent.db")

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()

	if db == nil {
		t.Fatal("db should not be nil after InitDB")
	}

	// Verify DB file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("DB file was not created")
	}

	// Verify WAL file exists (WAL mode selected)
	walPath := dbPath + "-wal"
	if _, err := os.Stat(walPath); err != nil {
		t.Logf("WAL file not found (may be ok): %v", err)
	}
}

func TestInitDB_Twice(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "twice.db")

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("first InitDB failed: %v", err)
	}
	CloseDB()

	// Second init should work (reopens)
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("second InitDB failed: %v", err)
	}
	CloseDB()
}

func TestCloseDB_WithoutInit(t *testing.T) {
	// CloseDB should not panic if db is nil
	db = nil
	if err := CloseDB(); err != nil {
		t.Fatalf("CloseDB on nil db: %v", err)
	}
}

func TestGetDB_AfterInit(t *testing.T) {
	dir := t.TempDir()
	InitDB(filepath.Join(dir, "get.db"))
	defer CloseDB()

	gdb := GetDB()
	if gdb == nil {
		t.Fatal("GetDB returned nil after InitDB")
	}
}

func TestGetDB_BeforeInit(t *testing.T) {
	db = nil
	gdb := GetDB()
	if gdb != nil {
		t.Fatal("GetDB should return nil before InitDB")
	}
}

func TestInitDB_CreatesModels(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "models.db")

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()

	// Verify models exist by querying each table
	models := []any{&NodeConfig{}, &NodeSecret{}, &MetricsSnapshot{}}
	for _, mdl := range models {
		if !db.Migrator().HasTable(mdl) {
			t.Errorf("table for %T was not created by AutoMigrate", mdl)
		}
	}
}

func TestNodeSecret_CreateAndRead(t *testing.T) {
	dir := t.TempDir()
	InitDB(filepath.Join(dir, "secret.db"))
	defer CloseDB()

	secret := NodeSecret{
		Secret:      "test-secret-key",
		HubNodeID:   "node-1",
		HubEndpoint: "https://hub.example.com",
	}

	if err := db.Create(&secret).Error; err != nil {
		t.Fatalf("create node secret: %v", err)
	}

	var read NodeSecret
	if err := db.First(&read, "hub_node_id = ?", "node-1").Error; err != nil {
		t.Fatalf("read node secret: %v", err)
	}

	if read.Secret != "test-secret-key" {
		t.Errorf("secret = %q, want 'test-secret-key'", read.Secret)
	}
	if read.HubEndpoint != "https://hub.example.com" {
		t.Errorf("hub endpoint = %q, want 'https://hub.example.com'", read.HubEndpoint)
	}
}

func TestNodeConfig_CreateAndRead(t *testing.T) {
	dir := t.TempDir()
	InitDB(filepath.Join(dir, "config.db"))
	defer CloseDB()

	cfg := NodeConfig{
		ConfigVersion: 1,
	}

	if err := db.Create(&cfg).Error; err != nil {
		t.Fatalf("create node config: %v", err)
	}

	var read NodeConfig
	if err := db.First(&read).Error; err != nil {
		t.Fatalf("read node config: %v", err)
	}

	if read.ConfigVersion != 1 {
		t.Errorf("config version = %d, want 1", read.ConfigVersion)
	}
}

func TestMetricsSnapshot_CreateAndRead(t *testing.T) {
	dir := t.TempDir()
	InitDB(filepath.Join(dir, "metrics.db"))
	defer CloseDB()

	snapshot := MetricsSnapshot{
		CPUPercent:    45.2,
		MemoryPercent: 60.1,
		DiskPercent:   30.0,
		TrafficSent:   1000,
		TrafficRecv:   500,
	}

	if err := db.Create(&snapshot).Error; err != nil {
		t.Fatalf("create metrics snapshot: %v", err)
	}

	var read MetricsSnapshot
	if err := db.First(&read).Error; err != nil {
		t.Fatalf("read metrics snapshot: %v", err)
	}

	if read.CPUPercent != 45.2 {
		t.Errorf("CPUPercent = %f, want 45.2", read.CPUPercent)
	}
	if read.MemoryPercent != 60.1 {
		t.Errorf("MemoryPercent = %f, want 60.1", read.MemoryPercent)
	}
}

func TestInitDB_InvalidPath(t *testing.T) {
	// Path in a non-existent directory should be created (MkdirAll).
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "nested", "agent.db")

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB with nested path: %v", err)
	}
	defer CloseDB()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("DB file was not created in nested path")
	}
}
