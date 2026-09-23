package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/auth"
)

// SessionChecker confirms a parsed cookie still corresponds to a live session —
// i.e. the admin exists and its token version has not been bumped by a password
// change. Pass nil to skip the check (tests).
type SessionChecker func(username string, ver int) bool

func RequireAuth(jm *auth.JWTManager, valid SessionChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		username, ver, err := jm.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if valid != nil && !valid(username, ver) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("username", username)
		c.Next()
	}
}
