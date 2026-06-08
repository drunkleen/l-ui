package service

import (
	"encoding/json"
	"errors"
	"sync"

	agentdb "github.com/drunkleen/l-ui/agent/database"
	"gorm.io/gorm"
)

type NodeConfigData struct {
	HubNodeID     string          `json:"hub_node_id"`
	HubEndpoint   string          `json:"hub_endpoint"`
	XrayConfig    json.RawMessage `json:"xray_config"`
	ClientList    json.RawMessage `json:"client_list"`
	ConfigVersion int             `json:"config_version"`
}

type ConfigService struct {
	mu sync.Mutex
}

func NewConfigService() *ConfigService {
	return &ConfigService{}
}

func (s *ConfigService) GetConfig() (*NodeConfigData, error) {
	db := agentdb.GetDB()
	if db == nil {
		return nil, nil
	}
	var cfg agentdb.NodeConfig
	result := db.Last(&cfg)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	data := &NodeConfigData{
		HubNodeID:     cfg.HubNodeID,
		HubEndpoint:   cfg.HubEndpoint,
		ConfigVersion: cfg.ConfigVersion,
	}
	if cfg.XrayConfig != "" {
		data.XrayConfig = json.RawMessage(cfg.XrayConfig)
	}
	if cfg.ClientList != "" {
		data.ClientList = json.RawMessage(cfg.ClientList)
	}
	return data, nil
}

func (s *ConfigService) PushConfig(hubNodeID, hubEndpoint string, xrayConfig json.RawMessage, clientList json.RawMessage) error {
	db := agentdb.GetDB()
	if db == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var cfg agentdb.NodeConfig
	result := db.Last(&cfg)
	if result.Error != nil {
		cfg = agentdb.NodeConfig{
			HubNodeID:     hubNodeID,
			HubEndpoint:   hubEndpoint,
			XrayConfig:    string(xrayConfig),
			ClientList:    string(clientList),
			ConfigVersion: 1,
		}
		return db.Create(&cfg).Error
	}
	cfg.HubNodeID = hubNodeID
	cfg.HubEndpoint = hubEndpoint
	cfg.XrayConfig = string(xrayConfig)
	cfg.ClientList = string(clientList)
	cfg.ConfigVersion++
	return db.Save(&cfg).Error
}

func (s *ConfigService) GetConfigVersion() int {
	db := agentdb.GetDB()
	if db == nil {
		return 0
	}
	var cfg agentdb.NodeConfig
	result := db.Last(&cfg)
	if result.Error != nil {
		return 0
	}
	return cfg.ConfigVersion
}
