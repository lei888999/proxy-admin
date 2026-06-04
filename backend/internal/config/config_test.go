package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	c := Load()
	if c.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", c.Port)
	}
	if c.DBPath != "sing-box-admin.db" {
		t.Fatalf("DBPath = %q", c.DBPath)
	}
	if c.DefaultAdminUser != "admin" || c.DefaultAdminPass != "mnice7082" {
		t.Fatalf("default admin = %q/%q", c.DefaultAdminUser, c.DefaultAdminPass)
	}
	if c.JWTSecret == "" {
		t.Fatal("JWTSecret should be auto-generated when unset")
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("PORT", "9000")
	os.Setenv("JWT_SECRET", "fixed-secret")
	c := Load()
	if c.Port != "9000" {
		t.Fatalf("Port = %q, want 9000", c.Port)
	}
	if c.JWTSecret != "fixed-secret" {
		t.Fatalf("JWTSecret = %q", c.JWTSecret)
	}
}
