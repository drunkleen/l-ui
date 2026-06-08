package service

import (
	"encoding/json"

	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/xray"
)

func renderXrayConfig(templateConfig string, inbounds []*model.Inbound) (*xray.Config, error) {
	xrayConfig := &xray.Config{}
	if err := json.Unmarshal([]byte(templateConfig), xrayConfig); err != nil {
		return nil, err
	}
	xrayConfig.LogConfig = resolveXrayLogPaths(xrayConfig.LogConfig)

	for _, inbound := range inbounds {
		if inbound == nil || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		settings := map[string]any{}
		_ = json.Unmarshal([]byte(inbound.Settings), &settings)
		if len(inbound.StreamSettings) > 0 {
			var stream map[string]any
			_ = json.Unmarshal([]byte(inbound.StreamSettings), &stream)
			delete(stream, "externalProxy")
			if raw, err := json.MarshalIndent(stream, "", "  "); err == nil {
				inbound.StreamSettings = string(raw)
			}
		}
		if inbound.Protocol == model.Shadowsocks {
			if healed, ok := model.HealShadowsocksClientMethods(inbound.Settings); ok {
				inbound.Settings = healed
			}
		}
		if len(inbound.Settings) > 0 {
			if raw, err := json.MarshalIndent(settings, "", "  "); err == nil {
				inbound.Settings = string(raw)
			}
		}
		inboundConfig := inbound.GenXrayInboundConfig()
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}
	return xrayConfig, nil
}

func (s *ServerService) RenderConfigJson() (any, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	xrayConfig, err := renderXrayConfig(templateConfig, inbounds)
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(xrayConfig, "", "  ")
	if err != nil {
		return nil, err
	}
	var jsonData any
	if err := json.Unmarshal(contents, &jsonData); err != nil {
		return nil, err
	}
	return jsonData, nil
}
