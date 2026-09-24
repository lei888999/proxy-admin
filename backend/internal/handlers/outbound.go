package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
)

type OutboundController interface {
	ListOutboundViews() ([]inbound.OutboundView, error)
	CreateOutbound(typ, tag, server string, port uint16, username, password string) (models.Outbound, error)
	UpdateOutbound(id uint, typ, tag, server string, port uint16, username, password string) (models.Outbound, error)
	DeleteOutbound(id uint) error
}

type OutboundHandler struct{ ctrl OutboundController }

func NewOutboundHandler(ctrl OutboundController) *OutboundHandler {
	return &OutboundHandler{ctrl: ctrl}
}

type outboundBody struct {
	Type     string `json:"type"`
	Tag      string `json:"tag"`
	Server   string `json:"server"`
	Port     uint16 `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func writeOutboundErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, inbound.ErrInvalidType), errors.Is(err, inbound.ErrInvalidTag), errors.Is(err, inbound.ErrInvalidOutbound):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, inbound.ErrTagExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, inbound.ErrOutboundInUse):
		c.JSON(http.StatusConflict, gin.H{"error": "出站仍被用户使用，请先改为其他出站或直连"})
	case errors.Is(err, inbound.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *OutboundHandler) List(c *gin.Context) {
	views, err := h.ctrl.ListOutboundViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, views)
}

func (h *OutboundHandler) Create(c *gin.Context) {
	var b outboundBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	o, err := h.ctrl.CreateOutbound(b.Type, b.Tag, b.Server, b.Port, b.Username, b.Password)
	respondMutation(c, o, err, writeOutboundErr)
}

func (h *OutboundHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var b outboundBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	o, err := h.ctrl.UpdateOutbound(id, b.Type, b.Tag, b.Server, b.Port, b.Username, b.Password)
	respondMutation(c, o, err, writeOutboundErr)
}

func (h *OutboundHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	respondMutation(c, okBody(), h.ctrl.DeleteOutbound(id), writeOutboundErr)
}
