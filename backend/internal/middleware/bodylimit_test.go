package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func bodyLimitRouter(limit int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", MaxBody(limit), func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.String(http.StatusRequestEntityTooLarge, "too large")
			return
		}
		c.String(http.StatusOK, "ok")
	})
	return r
}

func postBody(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	r.ServeHTTP(w, req)
	return w
}

func TestMaxBodyAllowsSmallBody(t *testing.T) {
	w := postBody(bodyLimitRouter(64), "hello")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
}

// An unbounded body lets one request push arbitrary bytes through memory and
// onto disk via the config endpoint.
func TestMaxBodyRejectsOversizedBody(t *testing.T) {
	w := postBody(bodyLimitRouter(64), strings.Repeat("a", 4096))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %d, want 413", w.Code)
	}
}
