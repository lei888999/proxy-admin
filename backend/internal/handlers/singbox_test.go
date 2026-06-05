package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/singbox"
)

type fakeCtrl struct {
	status    singbox.Status
	startErr  error
	stopErr   error
	config    string
	saveErr   error
	savedWith string
}

func (f *fakeCtrl) Status() singbox.Status         { return f.status }
func (f *fakeCtrl) Start() (singbox.Status, error) { return f.status, f.startErr }
func (f *fakeCtrl) Stop() (singbox.Status, error)  { return f.status, f.stopErr }
func (f *fakeCtrl) GetConfig() (string, error)     { return f.config, nil }
func (f *fakeCtrl) SaveConfig(c string) error      { f.savedWith = c; return f.saveErr }

func newRouter(ctrl SingboxController) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewSingboxHandler(ctrl)
	r := gin.New()
	r.GET("/api/status", h.Status)
	r.POST("/api/singbox/start", h.Start)
	r.POST("/api/singbox/stop", h.Stop)
	r.GET("/api/singbox/config", h.GetConfig)
	r.PUT("/api/singbox/config", h.PutConfig)
	return r
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestStatusEndpoint(t *testing.T) {
	r := newRouter(&fakeCtrl{status: singbox.Status{Installed: true, Version: "1.13.13", HasConfig: true}})
	w := do(r, http.MethodGet, "/api/status", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"version":"1.13.13"`) || !strings.Contains(w.Body.String(), `"hasConfig":true`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestStartInvalidConfig(t *testing.T) {
	r := newRouter(&fakeCtrl{startErr: &singbox.InvalidConfigError{Output: "bad inbound"}})
	w := do(r, http.MethodPost, "/api/singbox/start", "")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "bad inbound") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	r := newRouter(&fakeCtrl{startErr: singbox.ErrAlreadyRunning})
	w := do(r, http.MethodPost, "/api/singbox/start", "")
	if w.Code != 409 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestStopNotRunning(t *testing.T) {
	r := newRouter(&fakeCtrl{stopErr: singbox.ErrNotRunning})
	w := do(r, http.MethodPost, "/api/singbox/stop", "")
	if w.Code != 409 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestPutConfigInvalidJSON(t *testing.T) {
	r := newRouter(&fakeCtrl{saveErr: singbox.ErrInvalidJSON})
	w := do(r, http.MethodPut, "/api/singbox/config", `{"content":"{bad"}`)
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestPutConfigOK(t *testing.T) {
	ctrl := &fakeCtrl{}
	r := newRouter(ctrl)
	w := do(r, http.MethodPut, "/api/singbox/config", `{"content":"{\"log\":{}}"}`)
	if w.Code != 200 || ctrl.savedWith != `{"log":{}}` {
		t.Fatalf("code=%d saved=%q", w.Code, ctrl.savedWith)
	}
}

func TestGetConfig(t *testing.T) {
	r := newRouter(&fakeCtrl{config: `{"log":{}}`})
	w := do(r, http.MethodGet, "/api/singbox/config", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"content"`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
