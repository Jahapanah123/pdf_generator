package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jahapanah123/pdf_generator/internal/config"
)

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

type Manager struct {
	cfg config.JWTConfig
}

func NewManager(cfg config.JWTConfig) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) GenerateTokenPair(userID, email string) (*TokenPair, error) {
	now := time.Now()

	accessClaims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.cfg.AccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "pdf-generator",
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).
		SignedString([]byte(m.cfg.AccessSecret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshClaims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.cfg.RefreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "pdf-generator",
		},
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).
		SignedString([]byte(m.cfg.RefreshSecret))
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessClaims.ExpiresAt.Unix(),
	}, nil
}

func (m *Manager) ValidateAccessToken(tokenStr string) (*Claims, error) {
	return m.validate(tokenStr, m.cfg.AccessSecret)
}

func (m *Manager) ValidateRefreshToken(tokenStr string) (*Claims, error) {
	return m.validate(tokenStr, m.cfg.RefreshSecret)
}

func (m *Manager) validate(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
