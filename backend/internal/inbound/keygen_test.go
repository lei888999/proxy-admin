package inbound

import (
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"testing"

	"golang.org/x/crypto/curve25519"
)

func TestRealityKeypairDerivesPublicFromPrivate(t *testing.T) {
	priv, pub, err := NewKeyGen().RealityKeypair()
	if err != nil {
		t.Fatalf("RealityKeypair: %v", err)
	}
	pb, err := base64.RawURLEncoding.DecodeString(priv)
	if err != nil || len(pb) != 32 {
		t.Fatalf("priv decode err=%v len=%d", err, len(pb))
	}
	pubBytes, err := base64.RawURLEncoding.DecodeString(pub)
	if err != nil || len(pubBytes) != 32 {
		t.Fatalf("pub decode err=%v len=%d", err, len(pubBytes))
	}
	want, _ := curve25519.X25519(pb, curve25519.Basepoint)
	if base64.RawURLEncoding.EncodeToString(want) != pub {
		t.Fatal("public key is not X25519(private, basepoint)")
	}
}

func TestUUIDv4Format(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if got := NewKeyGen().UUID(); !re.MatchString(got) {
		t.Fatalf("uuid %q not v4", got)
	}
}

func TestShortIDIs8Hex(t *testing.T) {
	s := NewKeyGen().ShortID()
	if len(s) != 8 {
		t.Fatalf("len %d, want 8", len(s))
	}
	if _, err := hex.DecodeString(s); err != nil {
		t.Fatalf("not hex: %v", err)
	}
}
