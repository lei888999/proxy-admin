package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
)

type fakeSub struct {
	yaml     string
	err      error
	gotHost  string
	gotToken string
}

func (f *fakeSub) UserSubscription(token, host string) (string, error) {
	f.gotToken = token
	f.gotHost = host
	return f.yaml, f.err
}

func TestSubscriptionServesYAML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSubscriptionHandler(&fakeSub{yaml: "proxies: []\n"}, "vps.example.com")
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
	h := NewSubscriptionHandler(&fakeSub{err: inbound.ErrNotFound}, "vps.example.com")
	r.GET("/sub/:token", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/sub/nope", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("code=%d, want 404", w.Code)
	}
}

func TestSubscriptionStripsPortFromRequestHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	fs := &fakeSub{yaml: "proxies: []\n"} // empty SERVER_HOST -> fall back to request Host
	h := NewSubscriptionHandler(fs, "")
	r.GET("/sub/:token", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/sub/abc", nil)
	req.Host = "panel.example.com:8080"
	r.ServeHTTP(w, req)

	if fs.gotHost != "panel.example.com" {
		t.Fatalf("host passed to builder = %q, want bare host without :8080", fs.gotHost)
	}
}

func TestSubscriptionStripsPortFromConfiguredHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	fs := &fakeSub{yaml: "proxies: []\n"}
	h := NewSubscriptionHandler(fs, "vps.example.com:8080") // misconfigured with a port
	r.GET("/sub/:token", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/sub/abc", nil)
	r.ServeHTTP(w, req)

	if fs.gotHost != "vps.example.com" {
		t.Fatalf("host = %q, want vps.example.com (port stripped)", fs.gotHost)
	}
}
