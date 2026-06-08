package service

import (
	"encoding/json"
	"fmt"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/xray"
)

func BuildNodePushPayload(nodeID int) (xrayConfig, clientList json.RawMessage, err error) {
	xc, err := buildNodeXrayConfig(nodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("build node xray config: %w", err)
	}
	xrayData, err := json.Marshal(xc)
	if err != nil {
		return nil, nil, err
	}

	cl, err := buildNodeClientList(nodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("build node client list: %w", err)
	}
	clientData, err := json.Marshal(cl)
	if err != nil {
		return nil, nil, err
	}
	return xrayData, clientData, nil
}

func buildNodeXrayConfig(nodeID int) (*xray.Config, error) {
	db := database.GetDB()
	var setting model.Setting
	if err := db.Where("key = ?", "xrayTemplateConfig").First(&setting).Error; err != nil {
		return nil, fmt.Errorf("xray config template not found: %w", err)
	}
	xrayConfig := &xray.Config{}
	if err := json.Unmarshal([]byte(setting.Value), xrayConfig); err != nil {
		return nil, err
	}

	var inbounds []*model.Inbound
	if err := db.Where("node_id = ? AND enable = ?", nodeID, true).Preload("ClientStats").Find(&inbounds).Error; err != nil {
		return nil, err
	}
	for _, inbound := range inbounds {
		settings := map[string]any{}
		_ = json.Unmarshal([]byte(inbound.Settings), &settings)

		var dbClients []model.Client
		if err := db.Where("inbound_id = ?", inbound.Id).Find(&dbClients).Error; err != nil {
			return nil, err
		}

		enableMap := make(map[string]bool, len(inbound.ClientStats))
		for _, cs := range inbound.ClientStats {
			enableMap[cs.Email] = cs.Enable
		}

		var finalClients []any
		for i := range dbClients {
			c := dbClients[i]
			if enable, exists := enableMap[c.Email]; exists && !enable {
				logger.Infof("Remove user %s due to expiration or traffic limit", c.Email)
				continue
			}
			if !c.Enable {
				continue
			}
			flow := c.Flow
			if flow == "xtls-rprx-vision-udp443" {
				flow = "xtls-rprx-vision"
			}
			entry := map[string]any{"email": c.Email}
			switch inbound.Protocol {
			case model.VLESS:
				if c.ID != "" {
					entry["id"] = c.ID
				}
				if flow != "" {
					entry["flow"] = flow
				}
				if c.Reverse != nil {
					entry["reverse"] = c.Reverse
				}
			case model.VMESS:
				if c.ID != "" {
					entry["id"] = c.ID
				}
				if c.Security != "" {
					entry["security"] = c.Security
				}
			case model.Trojan:
				if c.Password != "" {
					entry["password"] = c.Password
				}
				if flow != "" {
					entry["flow"] = flow
				}
			case model.Shadowsocks:
				if c.Password != "" {
					entry["password"] = c.Password
				}
			case model.Hysteria:
				if c.Auth != "" {
					entry["auth"] = c.Auth
				}
			}
			finalClients = append(finalClients, entry)
		}

		_, hadClients := settings["clients"]
		mutated := hadClients || len(finalClients) > 0
		if mutated {
			settings["clients"] = finalClients
		}

		if inboundCanHostFallbacks(inbound) {
			var fallbackRecords []model.InboundFallback
			if err := db.Where("inbound_id = ?", inbound.Id).Order("id ASC").Find(&fallbackRecords).Error; err == nil && len(fallbackRecords) > 0 {
				generic := make([]any, 0, len(fallbackRecords))
				for _, f := range fallbackRecords {
					generic = append(generic, f)
				}
				settings["fallbacks"] = generic
				mutated = true
			}
		}

		if mutated {
			modified, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return nil, err
			}
			inbound.Settings = string(modified)
		}

		if inbound.StreamSettings != "" {
			var stream map[string]any
			_ = json.Unmarshal([]byte(inbound.StreamSettings), &stream)
			tlsSettings, ok1 := stream["tlsSettings"].(map[string]any)
			realitySettings, ok2 := stream["realitySettings"].(map[string]any)
			if ok1 || ok2 {
				if ok1 {
					delete(tlsSettings, "settings")
				} else if ok2 {
					delete(realitySettings, "settings")
				}
			}
			delete(stream, "externalProxy")
			newStream, err := json.MarshalIndent(stream, "", "  ")
			if err != nil {
				return nil, err
			}
			inbound.StreamSettings = string(newStream)
		}

		if inbound.Protocol == model.Shadowsocks {
			if healed, ok := model.HealShadowsocksClientMethods(inbound.Settings); ok {
				inbound.Settings = healed
			}
		}

		inboundConfig := inbound.GenXrayInboundConfig()
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}
	return xrayConfig, nil
}

func buildNodeClientList(nodeID int) ([]map[string]any, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Where("node_id = ? AND enable = ?", nodeID, true).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	var allClients []map[string]any
	for _, inbound := range inbounds {
		var dbClients []model.Client
		if err := db.Where("inbound_id = ?", inbound.Id).Find(&dbClients).Error; err != nil {
			return nil, err
		}
		for i := range dbClients {
			c := dbClients[i]
			if !c.Enable {
				continue
			}
			entry := map[string]any{"email": c.Email}
			switch inbound.Protocol {
			case model.VLESS:
				if c.ID != "" {
					entry["id"] = c.ID
				}
				if c.Flow != "" {
					entry["flow"] = c.Flow
				}
			case model.VMESS:
				if c.ID != "" {
					entry["id"] = c.ID
				}
				if c.Security != "" {
					entry["security"] = c.Security
				}
			case model.Trojan:
				if c.Password != "" {
					entry["password"] = c.Password
				}
			case model.Shadowsocks:
				if c.Password != "" {
					entry["password"] = c.Password
				}
			case model.Hysteria:
				if c.Auth != "" {
					entry["auth"] = c.Auth
				}
			}
			allClients = append(allClients, entry)
		}
	}
	return allClients, nil
}
