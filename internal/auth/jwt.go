package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	ID uuid.UUID `json:"user_id"`
	jwt.RegisteredClaims
}

type Issuer struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewIssuer(secret []byte, acc time.Duration, ref time.Duration) *Issuer {
	return &Issuer{
		secret:     secret,
		accessTTL:  acc,
		refreshTTL: ref,
	}
}

func (t *Issuer) GenJWT(ID uuid.UUID) (string, error) {
	now := time.Now()
	claims := Claims{
		ID: ID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.accessTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(token)
}

func (t *Issuer) ValJWT(tokenStr string) (Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid token")
		}
		return t.secret, nil
	})
	if err != nil || !token.Valid {
		return *claims, fmt.Errorf("invalid token")
	}
	return *claims, nil
}

func (t *Issuer) RefreshToken() time.Duration {
	return t.refreshTTL
}

func NewRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
