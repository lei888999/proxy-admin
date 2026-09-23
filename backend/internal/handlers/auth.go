package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"singbox-admin/internal/auth"
	"singbox-admin/internal/models"
)

const (
	cookieMaxAge = 7 * 24 * 60 * 60 // 7 days, seconds
	// minPasswordLen is a floor, not a policy: the panel is internet-facing and
	// the seeded default is published in this repo's own README.
	minPasswordLen = 8

	loginMaxFailures = 5
	loginWindow      = 15 * time.Minute
	loginBlock       = 15 * time.Minute
)

type AuthHandler struct {
	db *gorm.DB
	jm *auth.JWTManager
	// secureCookie marks the session cookie Secure. Off by default because the
	// panel can be served over plain HTTP; it MUST be on behind TLS, otherwise a
	// 7-day admin session travels in cleartext.
	secureCookie bool
	throttle     *auth.Throttle
}

func NewAuthHandler(db *gorm.DB, jm *auth.JWTManager, secureCookie bool) *AuthHandler {
	return &AuthHandler{
		db:           db,
		jm:           jm,
		secureCookie: secureCookie,
		throttle:     auth.NewThrottle(loginMaxFailures, loginWindow, loginBlock),
	}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) setSessionCookie(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", token, maxAge, "/", "", h.secureCookie, true)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}
	// Throttle BEFORE bcrypt so a flood of guesses cannot burn the host's CPU.
	key := c.ClientIP()
	if wait, ok := h.throttle.Allow(key); !ok {
		secs := int(wait.Seconds()) + 1
		c.Header("Retry-After", strconv.Itoa(secs))
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": fmt.Sprintf("登录尝试过于频繁，请 %d 秒后重试", secs),
		})
		return
	}
	var admin models.Admin
	if err := h.db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
		h.throttle.Fail(key)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)) != nil {
		h.throttle.Fail(key)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	token, err := h.jm.Generate(admin.Username, admin.TokenVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}
	h.throttle.Reset(key)
	h.setSessionCookie(c, token, cookieMaxAge)
	c.JSON(http.StatusOK, gin.H{"username": admin.Username})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	h.setSessionCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

// ChangePassword rotates the admin password. Runs behind RequireAuth, so the
// subject comes from the session rather than the request body.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	username, ok := c.Get("username")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "oldPassword and newPassword required"})
		return
	}
	if len([]rune(req.NewPassword)) < minPasswordLen {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("新密码至少 %d 个字符", minPasswordLen),
		})
		return
	}
	if req.NewPassword == req.OldPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码不能与当前密码相同"})
		return
	}
	var admin models.Admin
	if err := h.db.Where("username = ?", username).First(&admin).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.OldPassword)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码不正确"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash failed"})
		return
	}
	// Bumping the version invalidates every cookie issued under the old password.
	nextVer := admin.TokenVersion + 1
	if err := h.db.Model(&admin).Updates(map[string]any{
		"password_hash": string(hash),
		"token_version": nextVer,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Re-issue for the caller so changing your own password does not log you out.
	token, err := h.jm.Generate(admin.Username, nextVer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}
	h.setSessionCookie(c, token, cookieMaxAge)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SessionValid reports whether a cookie's subject/version still match the stored
// admin. Wired into the auth middleware.
func SessionValid(db *gorm.DB) func(string, int) bool {
	return func(username string, ver int) bool {
		var admin models.Admin
		if err := db.Where("username = ?", username).First(&admin).Error; err != nil {
			return false
		}
		return admin.TokenVersion == ver
	}
}
