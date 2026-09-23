package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"strings"
)

type Config struct {
	Port             string
	DBPath           string
	JWTSecret        string
	DefaultAdminUser string
	DefaultAdminPass string
	SingboxDir       string
	SingboxBin       string
	ServerHost       string
	// CookieSecure marks the session cookie Secure. Enable it whenever the panel
	// is reachable over HTTPS (directly or behind a reverse proxy) — without it
	// the 7-day admin session is sent in cleartext.
	CookieSecure bool
	// TrustedProxies lists reverse proxies whose X-Forwarded-For may be believed.
	// Empty means trust none, so ClientIP is the real peer address; that matters
	// because login throttling keys on it and gin's default trusts everybody.
	TrustedProxies []string
}

func Load() *Config {
	c := &Config{
		Port:             getenv("PORT", "8080"),
		DBPath:           getenv("DB_PATH", "sing-box-admin.db"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		DefaultAdminUser: getenv("DEFAULT_ADMIN_USER", "admin"),
		DefaultAdminPass: getenv("DEFAULT_ADMIN_PASS", "mnice7082"),
		SingboxDir:       getenv("SINGBOX_DIR", "./singbox"),
		SingboxBin:       os.Getenv("SINGBOX_BIN"),
		ServerHost:       os.Getenv("SERVER_HOST"),
		CookieSecure:     boolenv("COOKIE_SECURE", false),
		TrustedProxies:   listenv("TRUSTED_PROXIES"),
	}
	if c.JWTSecret == "" {
		c.JWTSecret = randomHex(32)
		log.Println("WARN: JWT_SECRET not set, generated a random one (sessions reset on restart)")
	}
	return c
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func boolenv(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

// listenv parses a comma-separated env var, dropping blanks.
func listenv(key string) []string {
	raw := strings.Split(os.Getenv(key), ",")
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
