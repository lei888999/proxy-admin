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
	inbounds   []models.Inbound
	createErr  error
	deleteErr  error
	users      []models.User
	createUErr error
	deleteUErr error
	regenErr   error
	lastTag    string
	lastUName  string
}

func (f *fakeInbCtrl) ListInbounds() ([]models.Inbound, error) { return f.inbounds, nil }
func (f *fakeInbCtrl) CreateInbound(tag string, port uint16, hs string) (models.Inbound, error) {
	f.lastTag = tag
	return models.Inbound{ID: 1, Tag: tag, Port: port}, f.createErr
}
func (f *fakeInbCtrl) DeleteInbound(id uint) error              { return f.deleteErr }
func (f *fakeInbCtrl) ListUsers(id uint) ([]models.User, error) { return f.users, nil }
func (f *fakeInbCtrl) CreateUser(id uint, name string) (models.User, error) {
	f.lastUName = name
	return models.User{ID: 1, Name: name, UUID: "u"}, f.createUErr
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
	e.GET("/api/inbounds", h.ListInbounds)
	e.POST("/api/inbounds", h.CreateInbound)
	e.DELETE("/api/inbounds/:id", h.DeleteInbound)
	e.GET("/api/inbounds/:id/users", h.ListUsers)
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

func TestCreateInboundOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"tag":"v1","port":443,"handshake":"www.microsoft.com"}`)
	if w.Code != 200 || c.lastTag != "v1" {
		t.Fatalf("code=%d tag=%q body=%s", w.Code, c.lastTag, w.Body.String())
	}
}

func TestCreateInboundDuplicate(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{createErr: inbound.ErrTagExists}, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"tag":"v1","port":443,"handshake":"h"}`)
	if w.Code != 409 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestCreateInboundBadBody(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{}, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"tag":"","port":0}`)
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestDeleteInboundNotFoundHandler(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{deleteErr: inbound.ErrNotFound}, &fakeRestarter{}), http.MethodDelete, "/api/inbounds/9", "")
	if w.Code != 404 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestCreateInboundInvalidTagHandler(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{createErr: inbound.ErrInvalidTag}, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"tag":"x","port":1,"handshake":"h"}`)
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestCreateInboundPortInUseHandler(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{createErr: inbound.ErrPortInUse}, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"tag":"x","port":1,"handshake":"h"}`)
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
