package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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
