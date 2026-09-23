package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBody caps how much a handler will read from a request body. The config
// endpoint accepts arbitrary JSON, so without a ceiling one request can push
// hundreds of megabytes through memory and onto disk.
func MaxBody(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
