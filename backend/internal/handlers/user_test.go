package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
)

type fakeUserCtrl struct {
	views     []inbound.UserView
	createErr error
	lastName  string
	lastIDs   []uint
}

func (f *fakeUserCtrl) ListUserViews() ([]inbound.UserView, error) { return f.views, nil }
func (f *fakeUserCtrl) CreateUser(name string, ids []uint) (models.User, error) {
	f.lastName, f.lastIDs = name, ids
	return models.User{ID: 1, Name: name, UUID: "u", Password: "p"}, f.createErr
}
func (f *fakeUserCtrl) UpdateUser(id uint, name string, ids []uint) (models.User, error) {
	return models.User{ID: id, Name: name}, nil
}
func (f *fakeUserCtrl) ResetUserCreds(id uint) (models.User, error) {
	return models.User{ID: id, UUID: "new"}, nil
}
func (f *fakeUserCtrl) DeleteUser(id uint) error       { return nil }
func (f *fakeUserCtrl) ResetUserTraffic(id uint) error { return nil }

func userRouter(ctrl UserController) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewUserHandler(ctrl)
	e := gin.New()
	e.GET("/api/users", h.List)
	e.POST("/api/users", h.Create)
	e.PUT("/api/users/:id", h.Update)
	e.POST("/api/users/:id/reset", h.Reset)
	e.DELETE("/api/users/:id", h.Delete)
	return e
}

func ureq(e *gin.Engine, m, p, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(m, p, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, r)
	return w
}

func TestUserCreate(t *testing.T) {
	c := &fakeUserCtrl{}
	w := ureq(userRouter(c), http.MethodPost, "/api/users", `{"name":"alice","inboundIds":[1,2]}`)
	if w.Code != 200 || c.lastName != "alice" || len(c.lastIDs) != 2 {
		t.Fatalf("code=%d name=%q ids=%v", w.Code, c.lastName, c.lastIDs)
	}
}

func TestUserCreateInvalidName(t *testing.T) {
	w := ureq(userRouter(&fakeUserCtrl{createErr: inbound.ErrInvalidName}), http.MethodPost, "/api/users", `{"name":"x"}`)
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestUserList(t *testing.T) {
	c := &fakeUserCtrl{views: []inbound.UserView{{ID: 1, Name: "a", UUID: "u", Password: "p", InboundTags: []string{"v1"}}}}
	w := ureq(userRouter(c), http.MethodGet, "/api/users", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"password":"p"`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestUserReset(t *testing.T) {
	w := ureq(userRouter(&fakeUserCtrl{}), http.MethodPost, "/api/users/3/reset", "")
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
}
