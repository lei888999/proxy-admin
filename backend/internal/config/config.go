package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
)

type Config struct {
	Port             string
	DBPath           string
	JWTSecret        string
	DefaultAdminUser string
	DefaultAdminPass string
	SingboxDir       string
	SingboxBin       string
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

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
