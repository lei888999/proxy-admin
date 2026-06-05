package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
)

type SubscriptionProvider interface {
	UserSubscription(token, serverHost string) (string, error)
}

type SubscriptionHandler struct {
	provider   SubscriptionProvider
	serverHost string
}

func NewSubscriptionHandler(p SubscriptionProvider, serverHost string) *SubscriptionHandler {
	return &SubscriptionHandler{provider: p, serverHost: serverHost}
}

func (h *SubscriptionHandler) Get(c *gin.Context) {
	host := h.serverHost
	if host == "" {
		host = c.Request.Host
	}
	body, err := h.provider.UserSubscription(c.Param("token"), host)
	if err != nil {
		if errors.Is(err, inbound.ErrNotFound) {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.String(http.StatusInternalServerError, "error")
		return
	}
	c.Header("Content-Type", "text/yaml; charset=utf-8")
	c.String(http.StatusOK, body)
}
