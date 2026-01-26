package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	signingKey []byte
}

func NewTokenManager(signingKey string) (*TokenManager, error) {
	if signingKey == "" {
		return nil, errors.New("empty signing key")
	}
	return &TokenManager{signingKey: []byte(signingKey)}, nil
}

func (m *TokenManager) NewAccessToken(userID int, role string, ttl time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID, // Storing as int in claims (JSON number)
		"role": role,
		"exp":  time.Now().Add(ttl).Unix(),
	})

	return token.SignedString(m.signingKey)
}

func (m *TokenManager) NewRefreshToken(userID int, ttl time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(ttl).Unix(),
	})

	return token.SignedString(m.signingKey)
}
