package auth

import (
	"testing"
	"time"
)

func TestGenerateAndParse(t *testing.T) {
	m := NewJWTManager("secret", 7*24*time.Hour)
	token, err := m.Generate("admin", 3)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	username, ver, err := m.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if username != "admin" {
		t.Fatalf("username = %q, want admin", username)
	}
	if ver != 3 {
		t.Fatalf("ver = %d, want 3", ver)
	}
}

func TestParseRejectsBadSignature(t *testing.T) {
	token, _ := NewJWTManager("secret", time.Hour).Generate("admin", 1)
	if _, _, err := NewJWTManager("other", time.Hour).Parse(token); err == nil {
		t.Fatal("expected error for wrong signing key")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	m := NewJWTManager("secret", -time.Hour) // already expired
	token, _ := m.Generate("admin", 1)
	if _, _, err := m.Parse(token); err == nil {
		t.Fatal("expected error for expired token")
	}
}
