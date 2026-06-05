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

func TestSingboxDefaults(t *testing.T) {
	os.Clearenv()
	c := Load()
	if c.SingboxDir != "./singbox" {
		t.Fatalf("SingboxDir = %q, want ./singbox", c.SingboxDir)
	}
	if c.SingboxBin != "" {
		t.Fatalf("SingboxBin = %q, want empty", c.SingboxBin)
	}
}

func TestSingboxFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("SINGBOX_DIR", "/data/singbox")
	os.Setenv("SINGBOX_BIN", "/usr/local/bin/sing-box")
	c := Load()
	if c.SingboxDir != "/data/singbox" || c.SingboxBin != "/usr/local/bin/sing-box" {
		t.Fatalf("got %q / %q", c.SingboxDir, c.SingboxBin)
	}
}
