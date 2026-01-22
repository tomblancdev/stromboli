package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	// Secret is the key used to sign tokens
	Secret string
	// AccessExpiry is how long access tokens are valid (default: 24h)
	AccessExpiry time.Duration
	// RefreshExpiry is how long refresh tokens are valid (default: 7 days)
	RefreshExpiry time.Duration
}

// Claims represents JWT claims
type Claims struct {
	jwt.RegisteredClaims
	IsRefresh bool `json:"is_refresh,omitempty"`
}

// Default expiry times
const (
	DefaultAccessExpiry  = 24 * time.Hour
	DefaultRefreshExpiry = 7 * 24 * time.Hour
)

// GenerateToken creates a new JWT access token for the given subject
func GenerateToken(subject string, cfg JWTConfig) (string, error) {
	expiry := cfg.AccessExpiry
	if expiry == 0 {
		expiry = DefaultAccessExpiry
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
		IsRefresh: false,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// GenerateRefreshToken creates a new JWT refresh token for the given subject
func GenerateRefreshToken(subject string, cfg JWTConfig) (string, error) {
	expiry := cfg.RefreshExpiry
	if expiry == 0 {
		expiry = DefaultRefreshExpiry
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
		IsRefresh: true,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return signedToken, nil
}

// ValidateToken validates an access token and returns its claims
func ValidateToken(tokenString string, cfg JWTConfig) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: token expired", ErrInvalidToken)
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("%w: token is invalid", ErrInvalidToken)
	}

	// Ensure this is not a refresh token being used as access token
	if claims.IsRefresh {
		return nil, fmt.Errorf("%w: refresh token cannot be used for access", ErrInvalidToken)
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and returns its claims
func ValidateRefreshToken(tokenString string, cfg JWTConfig) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: refresh token expired", ErrInvalidToken)
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("%w: refresh token is invalid", ErrInvalidToken)
	}

	// Ensure this is a refresh token
	if !claims.IsRefresh {
		return nil, fmt.Errorf("%w: access token cannot be used for refresh", ErrInvalidToken)
	}

	return claims, nil
}
