package controller

import (
	"net/http"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/gin-gonic/gin"
)

// Healthz returns 200 if the process is running.
func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz checks the database connection and returns 200 if OK, 503 if not.
func Readyz(c *gin.Context) {
	db := database.GetDB()
	if db == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"status": "error", "msg": "database not initialized"})
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"status": "error", "msg": "failed to get database connection"})
		return
	}
	if err := sqlDB.Ping(); err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"status": "error", "msg": "database ping failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
