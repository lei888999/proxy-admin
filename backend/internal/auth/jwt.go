package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims carries the admin's token version alongside the standard claims, so a
// password change can invalidate cookies already issued.
type Claims struct {
	jwt.RegisteredClaims
	Ver int `json:"ver"`
}

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), ttl: ttl}
}

func (m *JWTManager) Generate(username string, ver int) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Ver: ver,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse validates the token and returns its subject and token version.
func (m *JWTManager) Parse(tokenString string) (string, int, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return "", 0, err
	}
	return claims.Subject, claims.Ver, nil
}
