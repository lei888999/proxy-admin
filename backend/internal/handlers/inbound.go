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
	ListInbounds() ([]models.Inbound, error)
	CreateInbound(tag string, port uint16, handshake string) (models.Inbound, error)
	DeleteInbound(id uint) error
	ListUsers(inboundID uint) ([]models.User, error)
	CreateUser(inboundID uint, name string) (models.User, error)
	DeleteUser(id uint) error
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

func (h *InboundHandler) ListInbounds(c *gin.Context) {
	ins, err := h.ctrl.ListInbounds()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ins)
}

type createInboundBody struct {
	Tag       string `json:"tag" binding:"required"`
	Port      uint16 `json:"port" binding:"required"`
	Handshake string `json:"handshake" binding:"required"`
}

func (h *InboundHandler) CreateInbound(c *gin.Context) {
	var b createInboundBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag/port/handshake required"})
		return
	}
	in, err := h.ctrl.CreateInbound(b.Tag, b.Port, b.Handshake)
	if err != nil {
		if errors.Is(err, inbound.ErrTagExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "tag exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, in)
}

func (h *InboundHandler) DeleteInbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := h.ctrl.DeleteInbound(id); err != nil {
		writeInboundError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *InboundHandler) ListUsers(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	us, err := h.ctrl.ListUsers(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, us)
}

type createUserBody struct {
	Name string `json:"name" binding:"required"`
}

func (h *InboundHandler) CreateUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var b createUserBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	u, err := h.ctrl.CreateUser(id, b.Name)
	if err != nil {
		writeInboundError(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *InboundHandler) DeleteUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := h.ctrl.DeleteUser(id); err != nil {
		writeInboundError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *InboundHandler) Apply(c *gin.Context) {
	if err := h.ctrl.Regenerate(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	st, err := h.restarter.Restart()
	if err != nil {
		writeStartError(c, err) // from singbox.go (same package)
		return
	}
	c.JSON(http.StatusOK, st)
}

func writeInboundError(c *gin.Context, err error) {
	if errors.Is(err, inbound.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
