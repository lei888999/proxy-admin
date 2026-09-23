package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
	"singbox-admin/internal/singbox"
)

type InboundController interface {
	ListInboundViews() ([]inbound.InboundView, error)
	Types() []inbound.TypeInfo
	CreateInbound(typ, tag string, port uint16, params map[string]any) (models.Inbound, error)
	UpdateInbound(id uint, tag string, port uint16, params map[string]any) (models.Inbound, error)
	ResetInboundKeys(id uint) (models.Inbound, error)
	DeleteInbound(id uint) error
	Regenerate() error
}

type Restarter interface {
	Restart() (singbox.Status, error)
}

type InboundHandler struct {
	ctrl      InboundController
	restarter Restarter
}

func NewInboundHandler(ctrl InboundController, r Restarter) *InboundHandler {
	return &InboundHandler{ctrl: ctrl, restarter: r}
}

func parseID(c *gin.Context) (uint, bool) {
	v, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(v), true
}

func (h *InboundHandler) ListTypes(c *gin.Context) { c.JSON(http.StatusOK, h.ctrl.Types()) }

func (h *InboundHandler) ListInbounds(c *gin.Context) {
	views, err := h.ctrl.ListInboundViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, views)
}

type inboundBody struct {
	Type   string         `json:"type"`
	Tag    string         `json:"tag" binding:"required"`
	Port   uint16         `json:"port" binding:"required"`
	Params map[string]any `json:"params"`
}

func writeInboundCreateErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, inbound.ErrUnknownType), errors.Is(err, inbound.ErrInvalidTag):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, inbound.ErrTagExists), errors.Is(err, inbound.ErrPortInUse), errors.Is(err, inbound.ErrPortReserved):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, inbound.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *InboundHandler) CreateInbound(c *gin.Context) {
	var b inboundBody
	if err := c.ShouldBindJSON(&b); err != nil || b.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type/tag/port required"})
		return
	}
	in, err := h.ctrl.CreateInbound(b.Type, b.Tag, b.Port, b.Params)
	respondMutation(c, in, err, writeInboundCreateErr)
}

func (h *InboundHandler) UpdateInbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var b inboundBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag/port required"})
		return
	}
	in, err := h.ctrl.UpdateInbound(id, b.Tag, b.Port, b.Params)
	respondMutation(c, in, err, writeInboundCreateErr)
}

func (h *InboundHandler) ResetKeys(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	in, err := h.ctrl.ResetInboundKeys(id)
	respondMutation(c, in, err, writeInboundCreateErr)
}

func (h *InboundHandler) DeleteInbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	respondMutation(c, okBody(), h.ctrl.DeleteInbound(id), func(c *gin.Context, err error) {
		if errors.Is(err, inbound.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	})
}

func (h *InboundHandler) Apply(c *gin.Context) {
	// Unlike the mutation endpoints, this IS the apply operation, so a failure
	// here is the user's answer — routed through writeStartError so an invalid
	// config comes back as 400 + the `sing-box check` output rather than a 500.
	if err := h.ctrl.Regenerate(); err != nil {
		writeStartError(c, err)
		return
	}
	st, err := h.restarter.Restart()
	if err != nil {
		writeStartError(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}
