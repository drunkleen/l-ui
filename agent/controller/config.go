package controller

import (
	"encoding/json"
	"net/http"

	"github.com/drunkleen/l-ui/agent/service"
	"github.com/gin-gonic/gin"
)

var cfgSvc = service.NewConfigService()

func (s *ConfigController) GetConfig(c *gin.Context) {
	cfg, err := cfgSvc.GetConfig()
	if err != nil {
		abortJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": cfg})
}

type pushConfigRequest struct {
	HubNodeID   string          `json:"hub_node_id"`
	HubEndpoint string          `json:"hub_endpoint"`
	XrayConfig  json.RawMessage `json:"xray_config"`
	ClientList  json.RawMessage `json:"client_list"`
}

func (s *ConfigController) PushConfig(c *gin.Context) {
	var req pushConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortJSONError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := cfgSvc.PushConfig(req.HubNodeID, req.HubEndpoint, req.XrayConfig, req.ClientList); err != nil {
		abortJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj": gin.H{
			"config_version": cfgSvc.GetConfigVersion(),
		},
	})
}

func (s *ConfigController) ApplyConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "msg": "config applied"})
}
