package database

import (
	"time"
)

type NodeConfig struct {
	ID            uint   `gorm:"primaryKey;autoIncrement"`
	HubNodeID     string `gorm:"uniqueIndex;size:64"`
	HubEndpoint   string `gorm:"size:256"`
	XrayConfig    string `gorm:"type:text"`
	ClientList    string `gorm:"type:text"`
	ConfigVersion int    `gorm:"default:0"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type NodeSecret struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	Secret      string `gorm:"size:128;not null"`
	HubNodeID   string `gorm:"uniqueIndex;size:64"`
	HubEndpoint string `gorm:"size:256"`
	CreatedAt   time.Time
	ExpiresAt   *time.Time
}

type MetricsSnapshot struct {
	ID            uint `gorm:"primaryKey;autoIncrement"`
	CPUPercent    float64
	MemoryPercent float64
	DiskPercent   float64
	Uptime        int64
	TrafficSent   int64
	TrafficRecv   int64
	RecordedAt    time.Time `gorm:"index"`
}
