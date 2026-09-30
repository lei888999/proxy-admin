package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
)

func TestRoutingAPIValidatesWithoutOverwritingSavedRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Meta{}); err != nil {
		t.Fatal(err)
	}
	// No writer: a client policy endpoint must never need the sing-box process.
	svc := inbound.NewService(db, nil)
	h := NewRoutingHandler(svc)
	r := gin.New()
	r.GET("/rules", h.List)
	r.PUT("/rules", h.Save)
	send := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/rules", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	good := `[{"type":"DOMAIN","value":"Example.com","policy":"REJECT"}]`
	if w := send(good); w.Code != http.StatusOK {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{`{"not":"an array"}`, `[{"type":"IP-CIDR","value":"invalid","policy":"节点"}]`, `[{"type":"DOMAIN","value":"example.com","policy":"server-only"}]`} {
		if w := send(body); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid input accepted: %d %s", w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/rules", nil))
	var saved []inbound.RouteRule
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].Value != "example.com" || saved[0].Policy != "REJECT" {
		t.Fatalf("invalid input overwrote saved rules: %+v", saved)
	}
	if w := send(`[]`); w.Code != http.StatusOK {
		t.Fatal("cannot clear rules")
	}
}
