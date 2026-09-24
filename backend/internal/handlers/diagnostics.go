package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/diagnostics"
	"singbox-admin/internal/inbound"
)

type DiagnosticsController interface {
	ListOutboundViews() ([]inbound.OutboundView, error)
}

type DiagnosticsHandler struct{ ctrl DiagnosticsController }

func NewDiagnosticsHandler(ctrl DiagnosticsController) *DiagnosticsHandler {
	return &DiagnosticsHandler{ctrl: ctrl}
}

func (h *DiagnosticsHandler) Outbounds(c *gin.Context) {
	views, err := h.ctrl.ListOutboundViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	c.JSON(http.StatusOK, gin.H{"scope": "vps-to-upstream-tcp", "probes": diagnostics.ProbeOutbounds(ctx, views)})
}
