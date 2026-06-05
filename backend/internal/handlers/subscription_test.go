package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
)

type fakeSub struct {
	yaml string
	err  error
}

func (f fakeSub) UserSubscription(token, host string) (string, error) { return f.yaml, f.err }

func TestSubscriptionServesYAML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSubscriptionHandler(fakeSub{yaml: "proxies: []\n"}, "vps.example.com")
	r.GET("/sub/:token", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/sub/abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/yaml; charset=utf-8" {
		t.Fatalf("content-type=%q", ct)
	}
	if w.Body.String() != "proxies: []\n" {
		t.Fatalf("body=%q", w.Body.String())
	}
}

func TestSubscriptionUnknownToken404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSubscriptionHandler(fakeSub{err: inbound.ErrNotFound}, "vps.example.com")
	r.GET("/sub/:token", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/sub/nope", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("code=%d, want 404", w.Code)
	}
}
