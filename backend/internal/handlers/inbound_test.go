package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
	"singbox-admin/internal/singbox"
)

type fakeInbCtrl struct {
	views     []inbound.InboundView
	types     []inbound.TypeInfo
	createErr error
	updateErr error
	resetErr  error
	deleteErr error
	regenErr  error
	lastType  string
	lastTag   string
	lastID    uint
}

func (f *fakeInbCtrl) ListInboundViews() ([]inbound.InboundView, error) { return f.views, nil }
func (f *fakeInbCtrl) Types() []inbound.TypeInfo                        { return f.types }
func (f *fakeInbCtrl) CreateInbound(typ, tag string, port uint16, p map[string]any) (models.Inbound, error) {
	f.lastType, f.lastTag = typ, tag
	return models.Inbound{ID: 1, Tag: tag, Type: typ, Port: port}, f.createErr
}
func (f *fakeInbCtrl) UpdateInbound(id uint, tag string, port uint16, p map[string]any) (models.Inbound, error) {
	f.lastID, f.lastTag = id, tag
	return models.Inbound{ID: id, Tag: tag, Port: port}, f.updateErr
}
func (f *fakeInbCtrl) ResetInboundKeys(id uint) (models.Inbound, error) {
	f.lastID = id
	return models.Inbound{ID: id}, f.resetErr
}
func (f *fakeInbCtrl) DeleteInbound(id uint) error { return f.deleteErr }
func (f *fakeInbCtrl) Regenerate() error           { return f.regenErr }

type fakeRestarter struct {
	status singbox.Status
	err    error
}

func (f *fakeRestarter) Restart() (singbox.Status, error) { return f.status, f.err }

func inbRouter(ctrl InboundController, r Restarter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewInboundHandler(ctrl, r)
	e := gin.New()
	e.GET("/api/inbound-types", h.ListTypes)
	e.GET("/api/inbounds", h.ListInbounds)
	e.POST("/api/inbounds", h.CreateInbound)
	e.PUT("/api/inbounds/:id", h.UpdateInbound)
	e.POST("/api/inbounds/:id/reset-keys", h.ResetKeys)
	e.DELETE("/api/inbounds/:id", h.DeleteInbound)
	e.POST("/api/singbox/apply", h.Apply)
	return e
}

func inbReq(e *gin.Engine, m, p, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(m, p, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, r)
	return w
}

func TestListTypes(t *testing.T) {
	c := &fakeInbCtrl{types: []inbound.TypeInfo{{Type: "hysteria2", Label: "Hysteria2", Network: "udp", DefaultPort: 443}}}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodGet, "/api/inbound-types", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "hysteria2") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCreateInboundOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"type":"hysteria2","tag":"h1","port":443,"params":{"upMbps":50}}`)
	if w.Code != 200 || c.lastType != "hysteria2" {
		t.Fatalf("code=%d type=%q", w.Code, c.lastType)
	}
}

func TestUpdateInboundOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodPut, "/api/inbounds/5", `{"tag":"v2","port":9443,"params":{"handshake":"x.com"}}`)
	if w.Code != 200 || c.lastID != 5 || c.lastTag != "v2" {
		t.Fatalf("code=%d id=%d tag=%q", w.Code, c.lastID, c.lastTag)
	}
}

func TestUpdateInboundConflict(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{updateErr: inbound.ErrTagExists}, &fakeRestarter{}), http.MethodPut, "/api/inbounds/5", `{"tag":"v2","port":1}`)
	if w.Code != 409 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestResetKeysOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodPost, "/api/inbounds/7/reset-keys", "")
	if w.Code != 200 || c.lastID != 7 {
		t.Fatalf("code=%d id=%d", w.Code, c.lastID)
	}
}

func TestApplyReturnsStatus(t *testing.T) {
	r := &fakeRestarter{status: singbox.Status{Running: true, Installed: true}}
	w := inbReq(inbRouter(&fakeInbCtrl{}, r), http.MethodPost, "/api/singbox/apply", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"running":true`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestApplyMapsSingboxError(t *testing.T) {
	r := &fakeRestarter{err: singbox.ErrNotInstalled}
	w := inbReq(inbRouter(&fakeInbCtrl{}, r), http.MethodPost, "/api/singbox/apply", "")
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}
