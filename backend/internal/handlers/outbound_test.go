package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
)

type fakeOutboundCtrl struct {
	err error
}

func (f *fakeOutboundCtrl) ListOutboundViews() ([]inbound.OutboundView, error) {
	return []inbound.OutboundView{{ID: 1, Tag: "proxyA", Type: "socks5"}}, nil
}
func (f *fakeOutboundCtrl) CreateOutbound(typ, tag, server string, port uint16, username, password string) (models.Outbound, error) {
	if f.err != nil {
		return models.Outbound{}, f.err
	}
	return models.Outbound{ID: 1, Tag: tag, Type: typ, Server: server, Port: port}, nil
}
func (f *fakeOutboundCtrl) UpdateOutbound(id uint, typ, tag, server string, port uint16, username, password string) (models.Outbound, error) {
	return models.Outbound{ID: id, Tag: tag}, f.err
}
func (f *fakeOutboundCtrl) DeleteOutbound(id uint) error { return f.err }

func TestOutboundListAndCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewOutboundHandler(&fakeOutboundCtrl{})
	r.GET("/api/outbounds", h.List)
	r.POST("/api/outbounds", h.Create)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/outbounds", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list code=%d", w.Code)
	}

	body := `{"type":"socks5","tag":"proxyA","server":"1.2.3.4","port":1080}`
	w = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/outbounds", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestOutboundCreateBadType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewOutboundHandler(&fakeOutboundCtrl{err: inbound.ErrInvalidType})
	r.POST("/api/outbounds", h.Create)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/outbounds", bytes.NewBufferString(`{"type":"ftp","tag":"x","server":"1.2.3.4","port":1}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d, want 400", w.Code)
	}
}
