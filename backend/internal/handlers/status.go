package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/service"
)

type StatusHandler struct {
	svc *service.SingboxService
}

func NewStatusHandler(svc *service.SingboxService) *StatusHandler {
	return &StatusHandler{svc: svc}
}

func (h *StatusHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Status())
}
