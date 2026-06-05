package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
)

type UserController interface {
	ListUserViews() ([]inbound.UserView, error)
	CreateUser(name string, inboundIDs []uint) (models.User, error)
	UpdateUser(id uint, name string, inboundIDs []uint) (models.User, error)
	ResetUserCreds(id uint) (models.User, error)
	DeleteUser(id uint) error
	ResetUserTraffic(id uint) error
}

type UserHandler struct{ ctrl UserController }

func NewUserHandler(ctrl UserController) *UserHandler { return &UserHandler{ctrl: ctrl} }

func (h *UserHandler) List(c *gin.Context) {
	views, err := h.ctrl.ListUserViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, views)
}

type userBody struct {
	Name       string `json:"name" binding:"required"`
	InboundIDs []uint `json:"inboundIds"`
}

func writeUserErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, inbound.ErrInvalidName):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
	case errors.Is(err, inbound.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *UserHandler) Create(c *gin.Context) {
	var b userBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	u, err := h.ctrl.CreateUser(b.Name, b.InboundIDs)
	if err != nil {
		writeUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var b userBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	u, err := h.ctrl.UpdateUser(id, b.Name, b.InboundIDs)
	if err != nil {
		writeUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) Reset(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	u, err := h.ctrl.ResetUserCreds(id)
	if err != nil {
		writeUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) ResetTraffic(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := h.ctrl.ResetUserTraffic(id); err != nil {
		writeUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := h.ctrl.DeleteUser(id); err != nil {
		writeUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
