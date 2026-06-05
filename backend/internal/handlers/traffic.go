package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/traffic"
)

type TrafficHandler struct {
	clashAddr   string
	clashSecret string
}

func NewTrafficHandler(clashAddr, clashSecret string) *TrafficHandler {
	return &TrafficHandler{clashAddr: clashAddr, clashSecret: clashSecret}
}

func (h *TrafficHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, traffic.ReadLive(h.clashAddr, h.clashSecret))
}
