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
	views      []inbound.InboundView
	types      []inbound.TypeInfo
	createErr  error
	deleteErr  error
	users      []models.User
	createUErr error
	deleteUErr error
	regenErr   error
	lastType   string
	lastTag    string
	lastUName  string
}

func (f *fakeInbCtrl) ListInboundViews() ([]inbound.InboundView, error) { return f.views, nil }
func (f *fakeInbCtrl) Types() []inbound.TypeInfo                        { return f.types }
func (f *fakeInbCtrl) CreateInbound(typ, tag string, port uint16, params map[string]any) (models.Inbound, error) {
	f.lastType, f.lastTag = typ, tag
	return models.Inbound{ID: 1, Tag: tag, Type: typ, Port: port}, f.createErr
}
func (f *fakeInbCtrl) DeleteInbound(id uint) error              { return f.deleteErr }
func (f *fakeInbCtrl) ListUsers(id uint) ([]models.User, error) { return f.users, nil }
func (f *fakeInbCtrl) CreateUser(id uint, name string) (models.User, error) {
	f.lastUName = name
	return models.User{ID: 1, Name: name, Credential: "c"}, f.createUErr
}
func (f *fakeInbCtrl) DeleteUser(id uint) error { return f.deleteUErr }
func (f *fakeInbCtrl) Regenerate() error        { return f.regenErr }

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
	e.DELETE("/api/inbounds/:id", h.DeleteInbound)
	e.POST("/api/inbounds/:id/users", h.CreateUser)
	e.DELETE("/api/users/:id", h.DeleteUser)
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
	if w.Code != 200 || c.lastType != "hysteria2" || c.lastTag != "h1" {
		t.Fatalf("code=%d type=%q tag=%q", w.Code, c.lastType, c.lastTag)
	}
}

func TestCreateInboundUnknownType(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{createErr: inbound.ErrUnknownType}, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"type":"x","tag":"t","port":1}`)
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestCreateInboundPortInUse(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{createErr: inbound.ErrPortInUse}, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"type":"vless-reality","tag":"t","port":443}`)
	if w.Code != 409 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestCreateUserOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodPost, "/api/inbounds/1/users", `{"name":"alice"}`)
	if w.Code != 200 || c.lastUName != "alice" {
		t.Fatalf("code=%d name=%q", w.Code, c.lastUName)
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
