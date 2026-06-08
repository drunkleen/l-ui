package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
)

func setupRegistrationTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "l-ui-registration-test.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGenerateToken(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	token, err := s.GenerateToken("test-node", "192.168.1.100", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if token.NodeName != "test-node" {
		t.Fatalf("NodeName = %q, want %q", token.NodeName, "test-node")
	}
	if token.NodeAddress != "192.168.1.100" {
		t.Fatalf("NodeAddress = %q, want %q", token.NodeAddress, "192.168.1.100")
	}
	if token.ConsumedAt != 0 {
		t.Fatal("expected fresh token to have ConsumedAt == 0")
	}
	if token.ExpiresAt <= time.Now().UnixMilli() {
		t.Fatal("expected token to expire in the future")
	}
}

func TestGenerateTokenEmptyName(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	token, err := s.GenerateToken("", "", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token.Token == "" {
		t.Fatal("expected non-empty token even with empty name/addr")
	}
}

func TestGenerateTokenNameTooLong(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	longName := make([]byte, 200)
	for i := range longName {
		longName[i] = 'a'
	}
	_, err := s.GenerateToken(string(longName), "", time.Hour)
	if err != ErrTokenNameTooLong {
		t.Fatalf("expected ErrTokenNameTooLong, got %v", err)
	}
}

func TestGenerateTokenDefaultTTL(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	token, err := s.GenerateToken("test", "addr", 0)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	// Default is 24h, should be within a reasonable range
	if token.ExpiresAt <= time.Now().Add(23*time.Hour).UnixMilli() {
		t.Fatal("expected default TTL of at least 24h")
	}
}

func TestValidateToken(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	created, err := s.GenerateToken("validate-test", "", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	validated, err := s.ValidateToken(created.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if validated.Id != created.Id {
		t.Fatalf("Id = %d, want %d", validated.Id, created.Id)
	}
}

func TestValidateTokenEmpty(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	if _, err := s.ValidateToken(""); err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestValidateTokenNotFound(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	if _, err := s.ValidateToken("nonexistent-token"); err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestValidateTokenConsumed(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	created, err := s.GenerateToken("consume-test", "", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ConsumeToken(created.Token, 42); err != nil {
		t.Fatalf("ConsumeToken failed: %v", err)
	}

	if _, err := s.ValidateToken(created.Token); err != ErrTokenConsumed {
		t.Fatalf("expected ErrTokenConsumed, got %v", err)
	}
}

func TestValidateTokenExpired(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	db := database.GetDB()
	expired := &model.NodeRegistrationToken{
		Token:     "already-expired-validate",
		NodeName:  "expired",
		ExpiresAt: time.Now().Add(-time.Hour).UnixMilli(),
	}
	if err := db.Create(expired).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := s.ValidateToken(expired.Token); err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestConsumeToken(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	created, err := s.GenerateToken("consume", "", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ConsumeToken(created.Token, 7); err != nil {
		t.Fatalf("ConsumeToken failed: %v", err)
	}

	db := database.GetDB()
	var refreshed model.NodeRegistrationToken
	if err := db.First(&refreshed, created.Id).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.ConsumedByNodeID != 7 {
		t.Fatalf("ConsumedByNodeID = %d, want 7", refreshed.ConsumedByNodeID)
	}
	if refreshed.ConsumedAt == 0 {
		t.Fatal("expected ConsumedAt to be set")
	}
}

func TestConsumeTokenInvalid(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	if err := s.ConsumeToken("", 0); err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestConsumeTokenNotFound(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	if err := s.ConsumeToken("does-not-exist", 0); err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestListTokens(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	if _, err := s.GenerateToken("node-a", "10.0.0.1", time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GenerateToken("node-b", "10.0.0.2", time.Hour); err != nil {
		t.Fatal(err)
	}

	tokens, err := s.ListTokens()
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2", len(tokens))
	}
}

func TestDeleteToken(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	created, err := s.GenerateToken("delete-test", "", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteToken(created.Id); err != nil {
		t.Fatalf("DeleteToken failed: %v", err)
	}

	db := database.GetDB()
	var count int64
	db.Model(&model.NodeRegistrationToken{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 tokens after delete, got %d", count)
	}
}

func TestDeleteTokenNotFound(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	if err := s.DeleteToken(999999); err == nil {
		t.Fatal("expected error for non-existent token")
	}
}

func TestCleanupExpired(t *testing.T) {
	setupRegistrationTestDB(t)
	s := &RegistrationService{}

	// Create an expired token
	if _, err := s.GenerateToken("fresh", "", time.Hour); err != nil {
		t.Fatal(err)
	}
	// Create a token that expired in the past
	genDb := database.GetDB()
	expired := &model.NodeRegistrationToken{
		Token:     "already-expired",
		NodeName:  "old",
		ExpiresAt: time.Now().Add(-time.Hour).UnixMilli(),
	}
	if err := genDb.Create(expired).Error; err != nil {
		t.Fatal(err)
	}

	deleted, err := s.CleanupExpired()
	if err != nil {
		t.Fatalf("CleanupExpired failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}
}
