package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/traffic"
)

// LiveTrafficSource exposes the single cached, global throughput sample.
type LiveTrafficSource interface {
	Snapshot() traffic.Live
}

type TrafficHandler struct{ source LiveTrafficSource }

func NewTrafficHandler(source LiveTrafficSource) *TrafficHandler {
	return &TrafficHandler{source: source}
}

func (h *TrafficHandler) Live(c *gin.Context) {
	// This is only an in-memory read. The monitor owns the one long-lived Clash
	// stream, so a page refresh never opens another stream at sing-box.
	c.JSON(http.StatusOK, h.source.Snapshot())
}
