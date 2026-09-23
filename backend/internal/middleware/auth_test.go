package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/auth"
)

func setupWith(checker SessionChecker) (*gin.Engine, *auth.JWTManager) {
	gin.SetMode(gin.TestMode)
	jm := auth.NewJWTManager("secret", time.Hour)
	r := gin.New()
	r.GET("/protected", RequireAuth(jm, checker), func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString("username"))
	})
	return r, jm
}

func setup() (*gin.Engine, *auth.JWTManager) { return setupWith(nil) }

func get(r *gin.Engine, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "token", Value: token})
	}
	r.ServeHTTP(w, req)
	return w
}

func TestRejectsMissingCookie(t *testing.T) {
	r, _ := setup()
	if w := get(r, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}

func TestAcceptsValidCookie(t *testing.T) {
	r, jm := setup()
	token, _ := jm.Generate("admin", 1)
	w := get(r, token)
	if w.Code != http.StatusOK || w.Body.String() != "admin" {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}

// A cookie minted under an older token version must be refused, which is what
// makes a password change actually revoke sessions issued before it.
func TestRejectsStaleTokenVersion(t *testing.T) {
	current := 2
	r, jm := setupWith(func(_ string, ver int) bool { return ver == current })
	stale, _ := jm.Generate("admin", 1)
	if w := get(r, stale); w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401 for a superseded token version", w.Code)
	}
	fresh, _ := jm.Generate("admin", current)
	if w := get(r, fresh); w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 for the current token version", w.Code)
	}
}
