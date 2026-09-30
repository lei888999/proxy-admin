package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/inbound"
)

type RoutingController interface {
	RouteRules() ([]inbound.RouteRule, error)
	SaveRouteRules([]inbound.RouteRule) error
}

type RoutingHandler struct{ ctrl RoutingController }

func NewRoutingHandler(ctrl RoutingController) *RoutingHandler { return &RoutingHandler{ctrl: ctrl} }

func (h *RoutingHandler) List(c *gin.Context) {
	rules, err := h.ctrl.RouteRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (h *RoutingHandler) Save(c *gin.Context) {
	var rules []inbound.RouteRule
	if err := c.ShouldBindJSON(&rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "规则格式不正确"})
		return
	}
	if err := h.ctrl.SaveRouteRules(rules); err != nil {
		if errors.Is(err, inbound.ErrInvalidRouteRule) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
