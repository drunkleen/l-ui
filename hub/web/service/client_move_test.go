package service

import (
	"path/filepath"
	"testing"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
)

func TestFilterInboundIDsByNode(t *testing.T) {
	left := 1
	right := 2
	ids := []int{10, 11, 12, 13}
	lookup := map[int]*int{
		10: nil,
		11: &left,
		12: &right,
		13: &left,
	}

	if got := filterInboundIDsByNode(ids, lookup, 1); len(got) != 2 || got[0] != 11 || got[1] != 13 {
		t.Fatalf("filterInboundIDsByNode(node=1) = %v, want [11 13]", got)
	}
	if got := filterInboundIDsByNode(ids, lookup, 0); len(got) != 1 || got[0] != 10 {
		t.Fatalf("filterInboundIDsByNode(node=0) = %v, want [10]", got)
	}
}

func TestMoveNoopsWhenClientAlreadyOnTarget(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("LUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "l-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	left := 1
	right := 2

	if err := db.Create(&model.Node{Name: "node-a", Scheme: "http", Address: "127.0.0.1", Port: 30001, ApiToken: "a"}).Error; err != nil {
		t.Fatalf("create node a: %v", err)
	}
	targetNode := &model.Node{Name: "node-b", Scheme: "http", Address: "127.0.0.1", Port: 30002, ApiToken: "b"}
	if err := db.Create(targetNode).Error; err != nil {
		t.Fatalf("create node b: %v", err)
	}
	if err := db.Create(&model.Inbound{Tag: "target", Enable: true, Port: 31002, Protocol: model.VLESS, NodeID: &right, Settings: `{"clients":[]}`}).Error; err != nil {
		t.Fatalf("create target inbound: %v", err)
	}

	var targetInbound model.Inbound
	if err := db.Where("tag = ?", "target").First(&targetInbound).Error; err != nil {
		t.Fatalf("lookup target inbound: %v", err)
	}

	if err := db.Create(&model.ClientRecord{Email: "already@x", UUID: "uuid-1", Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	var rec model.ClientRecord
	if err := db.Where("email = ?", "already@x").First(&rec).Error; err != nil {
		t.Fatalf("lookup client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: targetInbound.Id}).Error; err != nil {
		t.Fatalf("link client to target: %v", err)
	}

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	if needRestart, err := svc.Move(inboundSvc, "already@x", left, right, targetInbound.Id); err != nil {
		t.Fatalf("Move returned error on idempotent no-op: %v", err)
	} else if needRestart {
		t.Fatal("idempotent no-op move should not require restart")
	}
}
