package controller

import (
	"net/http"
	"strconv"

	"github.com/drunkleen/l-ui/agent/service"
	"github.com/gin-gonic/gin"
)

var fwSvc = service.NewFirewallService()

func (s *FirewallController) GetStatus(c *gin.Context) {
	status, err := fwSvc.GetStatus()
	if err != nil {
		abortJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if status == nil {
		abortJSONError(c, http.StatusInternalServerError, "unable to determine firewall status")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": status})
}

func (s *FirewallController) GetRules(c *gin.Context) {
	rules, err := fwSvc.GetRules()
	if err != nil {
		abortJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if rules == nil {
		rules = []service.FirewallRule{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"rules": rules}})
}

type addRuleRequest struct {
	Port     string `json:"port" binding:"required"`
	Protocol string `json:"protocol"`
	Action   string `json:"action" binding:"required,oneof=allow deny reject limit"`
	Comment  string `json:"comment"`
}

func (s *FirewallController) AddRule(c *gin.Context) {
	var req addRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortJSONError(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.Port == "" {
		abortJSONError(c, http.StatusBadRequest, "port is required")
		return
	}
	if req.Protocol != "" && req.Protocol != "tcp" && req.Protocol != "udp" {
		abortJSONError(c, http.StatusBadRequest, "protocol must be tcp or udp")
		return
	}
	portInt, err := strconv.Atoi(req.Port)
	if err != nil || portInt < 1 || portInt > 65535 {
		abortJSONError(c, http.StatusBadRequest, "port must be a valid number between 1 and 65535")
		return
	}

	if err := fwSvc.AddRule(req.Port, req.Protocol, req.Action, req.Comment); err != nil {
		abortJSONError(c, http.StatusInternalServerError, "failed to add rule: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "msg": "rule added"})
}

type deleteRuleRequest struct {
	RuleNumber string `json:"rule_number" binding:"required"`
}

func (s *FirewallController) DeleteRule(c *gin.Context) {
	var req deleteRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortJSONError(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.RuleNumber == "" {
		abortJSONError(c, http.StatusBadRequest, "rule_number is required")
		return
	}

	if err := fwSvc.DeleteRule(req.RuleNumber); err != nil {
		abortJSONError(c, http.StatusInternalServerError, "failed to delete rule: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "msg": "rule deleted"})
}

func (s *FirewallController) Enable(c *gin.Context) {
	if err := fwSvc.Enable(); err != nil {
		abortJSONError(c, http.StatusInternalServerError, "failed to enable ufw: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "msg": "ufw enabled"})
}

func (s *FirewallController) Disable(c *gin.Context) {
	if err := fwSvc.Disable(); err != nil {
		abortJSONError(c, http.StatusInternalServerError, "failed to disable ufw: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "msg": "ufw disabled"})
}
