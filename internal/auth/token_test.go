package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateToken_CreatesValidToken(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
	}

	token, err := GenerateToken("user123", cfg)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Should be a valid JWT format (three parts separated by dots)
	assert.Contains(t, token, ".")
}

func TestGenerateToken_TokenContainsValidClaims(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
	}

	tokenString, err := GenerateToken("user123", cfg)
	require.NoError(t, err)

	// Parse token to verify claims
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})
	require.NoError(t, err)
	require.True(t, token.Valid)

	claims, ok := token.Claims.(*Claims)
	require.True(t, ok)
	assert.Equal(t, "user123", claims.Subject)
	assert.False(t, claims.IsRefresh)
	assert.WithinDuration(t, time.Now(), claims.IssuedAt.Time, 2*time.Second)
	assert.WithinDuration(t, time.Now().Add(time.Hour), claims.ExpiresAt.Time, 2*time.Second)
	assert.NotEmpty(t, claims.ID, "JTI claim should be set")
}

func TestGenerateToken_GeneratesUniqueJTI(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
	}

	// Generate two tokens
	token1, err := GenerateToken("user123", cfg)
	require.NoError(t, err)

	token2, err := GenerateToken("user123", cfg)
	require.NoError(t, err)

	// Parse both tokens
	claims1 := parseTokenClaims(t, token1, cfg)
	claims2 := parseTokenClaims(t, token2, cfg)

	// JTIs should be unique
	assert.NotEqual(t, claims1.ID, claims2.ID, "Each token should have a unique JTI")
}

func parseTokenClaims(t *testing.T, tokenString string, cfg JWTConfig) *Claims {
	t.Helper()
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})
	require.NoError(t, err)
	claims, ok := token.Claims.(*Claims)
	require.True(t, ok)
	return claims
}

func TestGenerateRefreshToken_CreatesValidRefreshToken(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	}

	tokenString, err := GenerateRefreshToken("user123", cfg)
	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	// Parse to verify it's a refresh token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})
	require.NoError(t, err)

	claims, ok := token.Claims.(*Claims)
	require.True(t, ok)
	assert.Equal(t, "user123", claims.Subject)
	assert.True(t, claims.IsRefresh)
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), claims.ExpiresAt.Time, 2*time.Second)
	assert.NotEmpty(t, claims.ID, "JTI claim should be set on refresh token")
}

func TestValidateToken_AcceptsValidToken(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
	}

	tokenString, err := GenerateToken("user123", cfg)
	require.NoError(t, err)

	claims, err := ValidateToken(tokenString, cfg)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, "user123", claims.Subject)
	assert.False(t, claims.IsRefresh)
}

func TestValidateToken_RejectsExpiredToken(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  -time.Hour, // Already expired
		RefreshExpiry: 24 * time.Hour,
	}

	tokenString, err := GenerateToken("user123", cfg)
	require.NoError(t, err)

	_, err = ValidateToken(tokenString, cfg)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateToken_RejectsInvalidSignature(t *testing.T) {
	cfg1 := JWTConfig{
		Secret:        "secret-1",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
	}

	cfg2 := JWTConfig{
		Secret:        "secret-2",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
	}

	tokenString, err := GenerateToken("user123", cfg1)
	require.NoError(t, err)

	// Try to validate with different secret
	_, err = ValidateToken(tokenString, cfg2)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateToken_RejectsMalformedToken(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
	}

	testCases := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"invalid format", "not.a.jwt.token"},
		{"random string", "random-string"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateToken(tc.token, cfg)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidToken)
		})
	}
}

func TestValidateToken_RejectsRefreshTokenAsAccessToken(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	}

	refreshToken, err := GenerateRefreshToken("user123", cfg)
	require.NoError(t, err)

	// ValidateToken should reject refresh tokens (they're for /auth/refresh only)
	_, err = ValidateToken(refreshToken, cfg)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateRefreshToken_AcceptsValidRefreshToken(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	}

	tokenString, err := GenerateRefreshToken("user123", cfg)
	require.NoError(t, err)

	claims, err := ValidateRefreshToken(tokenString, cfg)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, "user123", claims.Subject)
	assert.True(t, claims.IsRefresh)
}

func TestValidateRefreshToken_RejectsAccessToken(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	}

	accessToken, err := GenerateToken("user123", cfg)
	require.NoError(t, err)

	// ValidateRefreshToken should reject access tokens
	_, err = ValidateRefreshToken(accessToken, cfg)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateRefreshToken_RejectsExpiredRefreshToken(t *testing.T) {
	cfg := JWTConfig{
		Secret:        "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: -time.Hour, // Already expired
	}

	tokenString, err := GenerateRefreshToken("user123", cfg)
	require.NoError(t, err)

	_, err = ValidateRefreshToken(tokenString, cfg)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestJWTConfig_DefaultExpiry(t *testing.T) {
	cfg := JWTConfig{
		Secret: "test-secret-key",
	}

	// Should use defaults when not specified
	assert.Equal(t, time.Duration(0), cfg.AccessExpiry)
	assert.Equal(t, time.Duration(0), cfg.RefreshExpiry)

	// Test that we can still generate tokens (will use defaults in the function)
	token, err := GenerateToken("user123", cfg)
	require.NoError(t, err)
	require.NotEmpty(t, token)
}

func TestValidateToken_RejectsAlternativeSigningAlgorithms(t *testing.T) {
	cfg := JWTConfig{
		Secret:       "test-secret-key-must-be-long-enough",
		AccessExpiry: time.Hour,
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			ID:        "test-jti",
		},
	}

	cases := []struct {
		name   string
		method jwt.SigningMethod
	}{
		{"HS384 rejected", jwt.SigningMethodHS384},
		{"HS512 rejected", jwt.SigningMethodHS512},
		{"none algorithm rejected", jwt.SigningMethodNone},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := jwt.NewWithClaims(tc.method, claims)
			var (
				signed string
				err    error
			)
			if tc.method == jwt.SigningMethodNone {
				signed, err = token.SignedString(jwt.UnsafeAllowNoneSignatureType)
			} else {
				signed, err = token.SignedString([]byte(cfg.Secret))
			}
			require.NoError(t, err)

			_, err = ValidateToken(signed, cfg)
			require.Error(t, err, "validator must refuse non-HS256 tokens")
		})
	}
}
