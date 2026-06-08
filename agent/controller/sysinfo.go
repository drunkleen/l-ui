package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *SysInfoController) GetSysInfo(c *gin.Context) {
	info, err := sysSvc.GetInfo()
	if err != nil {
		abortJSONError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, info)
}
