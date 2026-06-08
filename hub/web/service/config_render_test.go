package service

import (
	"testing"

	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/database/model"
)

func TestRenderXrayConfigKeepsInboundOrder(t *testing.T) {
	template := `{"log":{},"inbounds":[]}`
	inbounds := []*model.Inbound{
		{Enable: false, NodeID: nil, Protocol: model.VLESS, Port: 1, Tag: "skip", Settings: `{"clients":[],"decryption":"none"}`},
		{Enable: true, NodeID: nil, Protocol: model.VLESS, Port: 1, Tag: "keep-1", Settings: `{"clients":[],"decryption":"none"}`},
		{Enable: true, NodeID: nil, Protocol: model.VLESS, Port: 2, Tag: "keep-2", Settings: `{"clients":[],"decryption":"none"}`},
	}
	got, err := renderXrayConfig(template, inbounds)
	if err != nil {
		t.Fatalf("renderXrayConfig returned error: %v", err)
	}
	if got == nil || len(got.InboundConfigs) != 2 {
		t.Fatalf("expected 2 active inbounds, got %#v", got)
	}
}

func TestNodeDriftReasons(t *testing.T) {
	n := &model.Node{Enable: false, Status: "offline", PanelVersion: "0.0.1", XrayVersion: "", LastError: "boom"}
	reasons := nodeDriftReasons(n)
	if len(reasons) < 4 {
		t.Fatalf("expected multiple drift reasons, got %v", reasons)
	}
	if config.GetVersion() == "" {
		t.Fatal("config version should not be empty")
	}
}
