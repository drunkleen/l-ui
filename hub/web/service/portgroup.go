package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/drunkleen/l-ui/hub/web/runtime"
	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/logger"
)

type PortGroupEntry struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Comment  string `json:"comment,omitempty"`
}

type PortGroupService struct{}

func NewPortGroupService() *PortGroupService {
	return &PortGroupService{}
}

func (s *PortGroupService) List() ([]model.PortGroup, error) {
	db := database.GetDB()
	var groups []model.PortGroup
	err := db.Model(model.PortGroup{}).Order("id asc").Find(&groups).Error
	return groups, err
}

func (s *PortGroupService) GetByID(id int) (*model.PortGroup, error) {
	db := database.GetDB()
	pg := &model.PortGroup{}
	if err := db.Model(model.PortGroup{}).Where("id = ?", id).First(pg).Error; err != nil {
		return nil, err
	}
	return pg, nil
}

func (s *PortGroupService) Create(name string, ports []PortGroupEntry) (*model.PortGroup, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	portsJSON, err := json.Marshal(ports)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ports: %w", err)
	}
	pg := &model.PortGroup{
		Name:  name,
		Ports: string(portsJSON),
	}
	db := database.GetDB()
	if err := db.Create(pg).Error; err != nil {
		return nil, err
	}
	return pg, nil
}

func (s *PortGroupService) Update(id int, name string, ports []PortGroupEntry) (*model.PortGroup, error) {
	portsJSON, err := json.Marshal(ports)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ports: %w", err)
	}
	updates := map[string]any{}
	if name != "" {
		updates["name"] = name
	}
	updates["ports"] = string(portsJSON)
	db := database.GetDB()
	if err := db.Model(model.PortGroup{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

func (s *PortGroupService) Delete(id int) error {
	db := database.GetDB()
	return db.Delete(model.PortGroup{}, id).Error
}

func (s *PortGroupService) PushToNodeGroup(portGroupID int, nodeGroup string) error {
	pg, err := s.GetByID(portGroupID)
	if err != nil {
		return fmt.Errorf("port group not found: %w", err)
	}
	var entries []PortGroupEntry
	if err := json.Unmarshal([]byte(pg.Ports), &entries); err != nil {
		return fmt.Errorf("invalid port group data: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}

	db := database.GetDB()
	var nodes []model.Node
	query := db.Model(model.Node{}).Where("enable = ?", true)
	if nodeGroup != "" {
		query = query.Where("group_name = ?", nodeGroup)
	}
	if err := query.Find(&nodes).Error; err != nil {
		return err
	}

	mgr := runtime.GetManager()
	if mgr == nil {
		return fmt.Errorf("runtime manager unavailable")
	}

	var lastErr error
	for _, n := range nodes {
		remote, err := mgr.RemoteFor(&n)
		if err != nil {
			logger.Warning("PushToNodeGroup: skipping node", n.Name, ":", err)
			lastErr = err
			continue
		}
		for _, entry := range entries {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := remote.AllowUfwPort(ctx, entry.Port, entry.Protocol); err != nil {
				logger.Warning("PushToNodeGroup: failed to allow port", entry.Port, "on node", n.Name, ":", err)
				lastErr = err
			}
			cancel()
		}
	}
	return lastErr
}
