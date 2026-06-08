package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Healthz returns 200 if the agent process is running.
func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz returns 200 since the agent has no database dependency.
// The agent is considered ready as long as it is running.
func Readyz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
