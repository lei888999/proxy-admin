package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
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
	// ClashAPIAddr optionally pins sing-box's internal Clash API to an unused
	// loopback address, e.g. 127.0.0.1:19090. It is never exposed publicly.
	ClashAPIAddr string
	ServerHost   string
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
		ClashAPIAddr:     loopbackTCPAddrEnv("CLASH_API_ADDR"),
		ServerHost:       os.Getenv("SERVER_HOST"),
		CookieSecure:     boolenv("COOKIE_SECURE", false),
		TrustedProxies:   listenv("TRUSTED_PROXIES"),
	}
	if c.JWTSecret == "" {
		if persisted, err := readPersistedJWTSecret(c.DBPath); err == nil && persisted != "" {
			c.JWTSecret = persisted
			log.Printf("WARN: JWT_SECRET not set, using the persisted session secret beside %s", c.DBPath)
		} else {
			c.JWTSecret = randomHex(32)
			log.Println("WARN: JWT_SECRET not set, generated a random one; it will be persisted beside the database")
		}
	}
	return c
}

// PersistJWTSecret keeps automatically generated session keys stable across
// restarts. Explicit JWT_SECRET remains the preferred deployment setting and
// is never written here.
func PersistJWTSecret(c *Config) error {
	if c == nil || c.JWTSecret == "" || os.Getenv("JWT_SECRET") != "" || isMemoryDB(c.DBPath) {
		return nil
	}
	path := jwtSecretPath(c.DBPath)
	if existing, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(existing)) != "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		if existing, readErr := os.ReadFile(path); readErr == nil && strings.TrimSpace(string(existing)) != "" {
			return os.Chmod(path, 0600)
		}
		f, err = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0600)
	}
	if err != nil {
		return err
	}
	if _, err := f.WriteString(c.JWTSecret + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func readPersistedJWTSecret(dbPath string) (string, error) {
	if isMemoryDB(dbPath) {
		return "", os.ErrNotExist
	}
	b, err := os.ReadFile(jwtSecretPath(dbPath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func jwtSecretPath(dbPath string) string { return dbPath + ".jwt-secret" }

func isMemoryDB(dbPath string) bool { return dbPath == "" || strings.Contains(dbPath, ":memory:") }

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// loopbackTCPAddrEnv accepts only a numeric loopback host with a valid TCP
// port. The Clash API is a panel-private control endpoint and must never bind
// to the VPS public address through an environment-variable typo.
func loopbackTCPAddrEnv(key string) string {
	addr := strings.TrimSpace(os.Getenv(key))
	if addr == "" {
		return ""
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.Printf("WARN: ignoring %s=%q: expected loopback host:port", key, addr)
		return ""
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		log.Printf("WARN: ignoring %s=%q: Clash API must bind to 127.0.0.1 or [::1]", key, addr)
		return ""
	}
	p, err := strconv.ParseUint(port, 10, 16)
	if err != nil || p == 0 {
		log.Printf("WARN: ignoring %s=%q: invalid TCP port", key, addr)
		return ""
	}
	return addr
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
