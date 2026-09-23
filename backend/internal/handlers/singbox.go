package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/singbox"
)

// SingboxController is the subset of *singbox.Service the handlers use.
type SingboxController interface {
	Status() singbox.Status
	Start() (singbox.Status, error)
	Stop() (singbox.Status, error)
	GetConfig() (string, error)
	SaveConfig(content string) error
}

type SingboxHandler struct{ ctrl SingboxController }

func NewSingboxHandler(c SingboxController) *SingboxHandler { return &SingboxHandler{ctrl: c} }

func (h *SingboxHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, h.ctrl.Status())
}

func (h *SingboxHandler) Start(c *gin.Context) {
	st, err := h.ctrl.Start()
	if err != nil {
		writeStartError(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

func (h *SingboxHandler) Stop(c *gin.Context) {
	st, err := h.ctrl.Stop()
	if err != nil {
		if errors.Is(err, singbox.ErrNotRunning) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, st)
}

func (h *SingboxHandler) GetConfig(c *gin.Context) {
	content, err := h.ctrl.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": content})
}

type configBody struct {
	Content string `json:"content"`
}

func (h *SingboxHandler) PutConfig(c *gin.Context) {
	var body configBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content required"})
		return
	}
	if err := h.ctrl.SaveConfig(body.Content); err != nil {
		if errors.Is(err, singbox.ErrInvalidJSON) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func writeStartError(c *gin.Context, err error) {
	var ice *singbox.InvalidConfigError
	var sfe *singbox.StartFailedError
	switch {
	case errors.As(err, &ice):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config", "detail": ice.Output})
	case errors.As(err, &sfe):
		// Spawned but died at once — almost always a listen port already taken.
		// The reason exists only in sing-box's log, so pass the tail through.
		c.JSON(http.StatusBadRequest, gin.H{"error": sfe.Error(), "detail": sfe.Output})
	case errors.Is(err, singbox.ErrNotInstalled), errors.Is(err, singbox.ErrNoConfig):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, singbox.ErrAlreadyRunning):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
