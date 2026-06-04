package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/auth"
	"singbox-admin/internal/middleware"
	"singbox-admin/internal/service"
)

type stubRunner struct{}

func (stubRunner) Version() (string, error) { return "sing-box version 1.9.0", nil }
func (stubRunner) IsRunning() bool          { return false }

func TestStatusEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewStatusHandler(service.NewSingboxService(stubRunner{}))
	r := gin.New()
	r.GET("/api/status", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"installed":true`) || !strings.Contains(body, `"version":"1.9.0"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

// TestStatusRouteIsProtected wires the status route behind RequireAuth exactly
// as main.go does, so the auth boundary on /api/status is covered by a test
// (not only by the production wiring).
func TestStatusRouteIsProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jm := auth.NewJWTManager("secret", time.Hour)
	h := NewStatusHandler(service.NewSingboxService(stubRunner{}))
	r := gin.New()
	r.GET("/api/status", middleware.RequireAuth(jm), h.Get)

	// No cookie -> 401.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie: code = %d, want 401", w.Code)
	}

	// Valid cookie -> 200.
	token, _ := jm.Generate("admin")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid cookie: code = %d, want 200", w.Code)
	}
}
