package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"singbox-admin/internal/auth"
	"singbox-admin/internal/database"
	"singbox-admin/internal/middleware"
	"singbox-admin/internal/models"
)

func newAuthRouterWith(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)
	db, err := database.Init(":memory:", "admin", "mnice7082")
	if err != nil {
		t.Fatalf("db init: %v", err)
	}
	jm := auth.NewJWTManager("secret", time.Hour)
	h := NewAuthHandler(db, jm, false)
	r := gin.New()
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/auth/logout", h.Logout)
	r.PUT("/api/auth/password", middleware.RequireAuth(jm, SessionValid(db)), h.ChangePassword)
	return r, db
}

func newAuthRouter(t *testing.T) *gin.Engine {
	r, _ := newAuthRouterWith(t)
	return r
}

func postJSON(r *gin.Engine, method, path, body string, cookie string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	r.ServeHTTP(w, req)
	return w
}

// sessionCookie logs in and returns the Cookie header value for later requests.
func sessionCookie(t *testing.T, r *gin.Engine, username, password string) string {
	w := postJSON(r, http.MethodPost, "/api/auth/login",
		fmt.Sprintf(`{"username":%q,"password":%q}`, username, password), "")
	if w.Code != http.StatusOK {
		t.Fatalf("login for cookie failed: %d %s", w.Code, w.Body.String())
	}
	sc := w.Header().Get("Set-Cookie")
	if i := strings.Index(sc, ";"); i > 0 {
		sc = sc[:i]
	}
	return sc
}

func TestLoginSuccessSetsCookie(t *testing.T) {
	r := newAuthRouter(t)
	w := postJSON(r, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"mnice7082"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "token=") {
		t.Fatalf("expected token cookie, got %q", w.Header().Get("Set-Cookie"))
	}
}

// Without COOKIE_SECURE the cookie stays non-Secure (plain-HTTP deployments);
// with it on, the flag must actually be set.
func TestLoginCookieSecureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Init(":memory:", "admin", "mnice7082")
	if err != nil {
		t.Fatalf("db init: %v", err)
	}
	r := gin.New()
	r.POST("/api/auth/login", NewAuthHandler(db, auth.NewJWTManager("s", time.Hour), true).Login)
	w := postJSON(r, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"mnice7082"}`, "")
	if !strings.Contains(w.Header().Get("Set-Cookie"), "Secure") {
		t.Fatalf("expected Secure cookie, got %q", w.Header().Get("Set-Cookie"))
	}
}

func TestLoginWrongPassword(t *testing.T) {
	r := newAuthRouter(t)
	w := postJSON(r, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"wrong"}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}

func TestLoginMissingFields(t *testing.T) {
	r := newAuthRouter(t)
	w := postJSON(r, http.MethodPost, "/api/auth/login", `{}`, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

// Repeated failures must be throttled — an unthrottled bcrypt endpoint is both a
// brute-force target and a CPU-exhaustion vector.
func TestLoginThrottlesRepeatedFailures(t *testing.T) {
	r := newAuthRouter(t)
	for i := 0; i < loginMaxFailures; i++ {
		w := postJSON(r, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"wrong"}`, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: code = %d, want 401", i+1, w.Code)
		}
	}
	w := postJSON(r, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"wrong"}`, "")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("code = %d, want 429 after %d failures", w.Code, loginMaxFailures)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected a Retry-After header on a throttled login")
	}
	// The lockout must hold even for the CORRECT password, otherwise it is no
	// defence at all.
	w = postJSON(r, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"mnice7082"}`, "")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("code = %d, want 429 while locked out", w.Code)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	r := newAuthRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("expected cookie cleared, got %q", w.Header().Get("Set-Cookie"))
	}
}

func TestChangePasswordSucceedsAndRotatesCredential(t *testing.T) {
	r := newAuthRouter(t)
	cookie := sessionCookie(t, r, "admin", "mnice7082")
	w := postJSON(r, http.MethodPut, "/api/auth/password",
		`{"oldPassword":"mnice7082","newPassword":"a-much-longer-secret"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	// Old password no longer works, new one does.
	if got := postJSON(r, http.MethodPost, "/api/auth/login",
		`{"username":"admin","password":"mnice7082"}`, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("old password still accepted: %d", got)
	}
	if got := postJSON(r, http.MethodPost, "/api/auth/login",
		`{"username":"admin","password":"a-much-longer-secret"}`, "").Code; got != http.StatusOK {
		t.Fatalf("new password rejected: %d", got)
	}
}

// Changing the password must invalidate sessions minted under the old one —
// otherwise rotating a leaked password locks nobody out for another 7 days.
func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	r, _ := newAuthRouterWith(t)
	victim := sessionCookie(t, r, "admin", "mnice7082")
	other := sessionCookie(t, r, "admin", "mnice7082")
	if w := postJSON(r, http.MethodPut, "/api/auth/password",
		`{"oldPassword":"mnice7082","newPassword":"a-much-longer-secret"}`, victim); w.Code != http.StatusOK {
		t.Fatalf("change password: %d %s", w.Code, w.Body.String())
	}
	w := postJSON(r, http.MethodPut, "/api/auth/password",
		`{"oldPassword":"a-much-longer-secret","newPassword":"yet-another-secret"}`, other)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("stale session code = %d, want 401", w.Code)
	}
}

func TestChangePasswordRejectsWrongOldPassword(t *testing.T) {
	r := newAuthRouter(t)
	cookie := sessionCookie(t, r, "admin", "mnice7082")
	w := postJSON(r, http.MethodPut, "/api/auth/password",
		`{"oldPassword":"nope","newPassword":"a-much-longer-secret"}`, cookie)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}

func TestChangePasswordRejectsShortPassword(t *testing.T) {
	r := newAuthRouter(t)
	cookie := sessionCookie(t, r, "admin", "mnice7082")
	w := postJSON(r, http.MethodPut, "/api/auth/password",
		`{"oldPassword":"mnice7082","newPassword":"short"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

func TestChangePasswordRequiresAuth(t *testing.T) {
	r := newAuthRouter(t)
	w := postJSON(r, http.MethodPut, "/api/auth/password",
		`{"oldPassword":"mnice7082","newPassword":"a-much-longer-secret"}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}

// The seeded admin must start at a usable token version, so a fresh install's
// first cookie is not immediately rejected by the session check.
func TestSeededAdminHasTokenVersion(t *testing.T) {
	_, db := newAuthRouterWith(t)
	var a models.Admin
	if err := db.Where("username = ?", "admin").First(&a).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}
	if a.TokenVersion < 1 {
		t.Fatalf("TokenVersion = %d, want >= 1", a.TokenVersion)
	}
}
