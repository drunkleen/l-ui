package service

import (
	"testing"
)

func TestNewSystemService(t *testing.T) {
	svc := NewSystemService()
	if svc == nil {
		t.Fatal("expected non-nil SystemService")
	}
}

func TestSystemService_GetMetrics_ReturnsValidStructure(t *testing.T) {
	svc := NewSystemService()
	metrics, err := svc.GetMetrics()
	if err != nil {
		t.Fatalf("GetMetrics() returned error: %v", err)
	}
	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	if metrics.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if metrics.CPUPercent < 0 {
		t.Error("expected non-negative CPU percent")
	}
	if metrics.MemoryTotal == 0 {
		t.Log("warning: memory total is 0 (may be running in constrained environment)")
	}
	if metrics.Uptime == 0 {
		t.Log("warning: uptime is 0")
	}
	_, ok := interface{}(metrics.Load1).(float64)
	if !ok {
		t.Error("expected Load1 to be float64")
	}
}

func TestSystemService_GetInfo_ReturnsValidStructure(t *testing.T) {
	svc := NewSystemService()
	info, err := svc.GetInfo()
	if err != nil {
		t.Fatalf("GetInfo() returned error: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.Hostname == "" {
		t.Log("warning: hostname is empty")
	}
	if info.OS == "" {
		t.Error("expected non-empty OS")
	}
	if info.KernelArch == "" {
		t.Error("expected non-empty kernel arch")
	}
	if info.CPUCores <= 0 {
		t.Error("expected positive CPU cores")
	}
	if info.GoVersion == "" {
		t.Error("expected non-empty Go version")
	}
}
