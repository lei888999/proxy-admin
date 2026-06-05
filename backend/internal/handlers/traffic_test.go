package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTrafficLiveReturnsSnapshot(t *testing.T) {
	clash := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"up":1000,"down":2000}` + "\n"))
	}))
	defer clash.Close()
	addr := clash.Listener.Addr().String()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewTrafficHandler(addr, "")
	r.GET("/api/traffic/live", h.Live)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/traffic/live", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	var body struct{ Up, Down int64 }
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Up != 1000 || body.Down != 2000 {
		t.Fatalf("body=%+v", body)
	}
}
