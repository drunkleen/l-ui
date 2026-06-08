package controller

import (
	"net/http"

	"github.com/drunkleen/l-ui/agent/service"
	"github.com/gin-gonic/gin"
)

var sysSvc = service.NewSystemService()

func (s *MetricsController) GetMetrics(c *gin.Context) {
	metrics, err := sysSvc.GetMetrics()
	if err != nil {
		abortJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, metrics)
}
