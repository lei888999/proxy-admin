package inbound

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// KeyGen produces the secrets needed for a VLESS+Reality inbound.
type KeyGen interface {
	RealityKeypair() (privateKey, publicKey string, err error)
	UUID() string
	ShortID() string
}

type goKeyGen struct{}

func NewKeyGen() KeyGen { return goKeyGen{} }

func (goKeyGen) RealityKeypair() (string, string, error) {
	priv := make([]byte, 32)
	if _, err := rand.Read(priv); err != nil {
		return "", "", err
	}
	// X25519 clamping (matches `sing-box generate reality-keypair`).
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(priv), enc.EncodeToString(pub), nil
}

func (goKeyGen) UUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (goKeyGen) ShortID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
