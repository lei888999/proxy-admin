package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/auth"
)

func setup() (*gin.Engine, *auth.JWTManager) {
	gin.SetMode(gin.TestMode)
	jm := auth.NewJWTManager("secret", time.Hour)
	r := gin.New()
	r.GET("/protected", RequireAuth(jm), func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString("username"))
	})
	return r, jm
}

func TestRejectsMissingCookie(t *testing.T) {
	r, _ := setup()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}

func TestAcceptsValidCookie(t *testing.T) {
	r, jm := setup()
	token, _ := jm.Generate("admin")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "admin" {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}
