package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestServesIndexForUnknownRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil) // client-side route
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (SPA fallback)", w.Code)
	}
}
