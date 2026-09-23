package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/traffic"
)

type fixedLive struct{ value traffic.Live }

func (f fixedLive) Snapshot() traffic.Live { return f.value }

// The handler must serve the monitor's in-memory snapshot. It must not know the
// Clash address or open a stream for every dashboard request.
func TestTrafficLiveReturnsCachedSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewTrafficHandler(fixedLive{value: traffic.Live{Up: 1000, Down: 2000}})
	r.GET("/api/traffic/live", h.Live)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/traffic/live", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	var body struct{ Up, Down int64 }
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Up != 1000 || body.Down != 2000 {
		t.Fatalf("body=%+v", body)
	}
}
