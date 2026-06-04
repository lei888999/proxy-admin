package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/auth"
	"singbox-admin/internal/database"
)

func newAuthRouter(t *testing.T) *gin.Engine {
	gin.SetMode(gin.TestMode)
	db, err := database.Init(":memory:", "admin", "mnice7082")
	if err != nil {
		t.Fatalf("db init: %v", err)
	}
	h := NewAuthHandler(db, auth.NewJWTManager("secret", time.Hour))
	r := gin.New()
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/auth/logout", h.Logout)
	return r
}

func TestLoginSuccessSetsCookie(t *testing.T) {
	r := newAuthRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"admin","password":"mnice7082"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "token=") {
		t.Fatalf("expected token cookie, got %q", w.Header().Get("Set-Cookie"))
	}
}

func TestLoginWrongPassword(t *testing.T) {
	r := newAuthRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"admin","password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}

func TestLoginMissingFields(t *testing.T) {
	r := newAuthRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	r := newAuthRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("expected cookie cleared, got %q", w.Header().Get("Set-Cookie"))
	}
}
