package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"singbox-admin/internal/singbox"
)

type ConnectionLogSource interface {
	ConnectionLogs() (singbox.ConnectionLogs, error)
}
type LogsHandler struct{ source ConnectionLogSource }

func NewLogsHandler(source ConnectionLogSource) *LogsHandler { return &LogsHandler{source: source} }
func (h *LogsHandler) List(c *gin.Context) {
	logs, err := h.source.ConnectionLogs()
	c.Header("Cache-Control", "no-store")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取连接日志失败"})
		return
	}
	c.JSON(http.StatusOK, logs)
}
