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

	// Use a path that is guaranteed never to exist as a generated static file,
	// so this asserts the SPA fallback regardless of what `make build` put in
	// dist/. (A real export creates dashboard/index.html etc., which would make
	// http.FileServer issue a 301 directory redirect instead.)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/totally-unknown-spa-route", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (SPA fallback)", w.Code)
	}
}
