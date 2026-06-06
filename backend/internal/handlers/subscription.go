package handlers

import (
	"errors"
	"net"
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

// hostOnly strips any :port from an authority so the subscription's proxy
// "server" is the bare host. The proxy port comes from each inbound, not from
// the panel's listen address (the request Host carries the panel port, e.g.
// :8080, which would make clients dial the wrong port and time out).
func hostOnly(authority string) string {
	if h, _, err := net.SplitHostPort(authority); err == nil {
		return h
	}
	return authority
}

func (h *SubscriptionHandler) Get(c *gin.Context) {
	host := h.serverHost
	if host == "" {
		host = c.Request.Host
	}
	host = hostOnly(host)
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
