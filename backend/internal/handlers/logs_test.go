package handlers

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/singbox"
)

type fakeLogSource struct{ err error }

func (f fakeLogSource) ConnectionLogs() (singbox.ConnectionLogs, error) {
	return singbox.ConnectionLogs{Entries: []singbox.ConnectionLog{}}, f.err
}
func TestConnectionLogsResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		err  error
		code int
	}{{nil, 200}, {errors.New("secret path"), 500}} {
		r := gin.New()
		r.GET("/logs", NewLogsHandler(fakeLogSource{tc.err}).List)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/logs", nil))
		if w.Code != tc.code || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("response: %d %s", w.Code, w.Body.String())
		}
		if tc.err != nil && w.Body.String() != `{"error":"读取连接日志失败"}` {
			t.Fatalf("internal error leaked: %s", w.Body.String())
		}
	}
}
