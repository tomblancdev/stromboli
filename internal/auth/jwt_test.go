package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter(cfg Config) *gin.Engine {
	router := gin.New()
	router.Use(Middleware(cfg))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	return router
}

func TestMiddleware_AuthDisabled_AllowsAllRequests(t *testing.T) {
	cfg := Config{
		Enabled:     false,
		ValidTokens: []string{},
	}
	router := setupTestRouter(cfg)

	req, err := http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["message"])
}

func TestMiddleware_MissingAuthHeader_Returns401(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ValidTokens: []string{"valid-token"},
	}
	router := setupTestRouter(cfg)

	req, err := http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, ErrMissingAuthHeader.Error(), response["error"])
}

func TestMiddleware_InvalidAuthFormat_NoBearerPrefix_Returns401(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ValidTokens: []string{"valid-token"},
	}
	router := setupTestRouter(cfg)

	req, err := http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "valid-token")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, ErrInvalidAuthHeader.Error(), response["error"])
}

func TestMiddleware_InvalidAuthFormat_WrongPrefix_Returns401(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ValidTokens: []string{"valid-token"},
	}
	router := setupTestRouter(cfg)

	req, err := http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Basic valid-token")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, ErrInvalidAuthHeader.Error(), response["error"])
}

func TestMiddleware_InvalidToken_Returns401(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ValidTokens: []string{"valid-token"},
	}
	router := setupTestRouter(cfg)

	req, err := http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer invalid-token")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, ErrInvalidToken.Error(), response["error"])
}

func TestMiddleware_ValidToken_AllowsRequest(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ValidTokens: []string{"valid-token"},
	}
	router := setupTestRouter(cfg)

	req, err := http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer valid-token")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["message"])
}

func TestMiddleware_CaseInsensitiveBearerPrefix(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ValidTokens: []string{"valid-token"},
	}
	router := setupTestRouter(cfg)

	testCases := []string{
		"Bearer valid-token",
		"bearer valid-token",
		"BEARER valid-token",
		"BeArEr valid-token",
	}

	for _, authHeader := range testCases {
		t.Run(authHeader, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/protected", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", authHeader)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)

			var response map[string]string
			err = json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, "success", response["message"])
		})
	}
}

func TestMiddleware_MultipleValidTokens(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ValidTokens: []string{"token-1", "token-2", "token-3"},
	}
	router := setupTestRouter(cfg)

	for _, token := range cfg.ValidTokens {
		t.Run(token, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/protected", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+token)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestMiddleware_EmptyToken_Returns401(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ValidTokens: []string{"valid-token"},
	}
	router := setupTestRouter(cfg)

	req, err := http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer ")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, ErrInvalidToken.Error(), response["error"])
}

func TestMiddleware_BlacklistedJWTToken_Returns401(t *testing.T) {
	blacklist := NewTokenBlacklist()
	jwtConfig := JWTConfig{
		Secret:       "test-secret-key",
		AccessExpiry: time.Hour,
	}

	cfg := Config{
		Enabled:   true,
		JWTConfig: jwtConfig,
		Blacklist: blacklist,
	}
	router := setupTestRouter(cfg)

	// Generate a valid JWT token
	tokenString, err := GenerateToken("user123", jwtConfig)
	require.NoError(t, err)

	// Verify token works before blacklisting
	req, err := http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "Token should work before blacklisting")

	// Parse token to get JTI
	claims, err := ValidateToken(tokenString, jwtConfig)
	require.NoError(t, err)
	require.NotEmpty(t, claims.ID)

	// Blacklist the token
	blacklist.Add(claims.ID, claims.ExpiresAt.Time)

	// Verify token is now rejected
	req, err = http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "Blacklisted token should be rejected")

	var response map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "token revoked", response["error"])
}

func TestMiddleware_NilBlacklist_AllowsJWTToken(t *testing.T) {
	jwtConfig := JWTConfig{
		Secret:       "test-secret-key",
		AccessExpiry: time.Hour,
	}

	cfg := Config{
		Enabled:   true,
		JWTConfig: jwtConfig,
		Blacklist: nil, // No blacklist configured
	}
	router := setupTestRouter(cfg)

	// Generate a valid JWT token
	tokenString, err := GenerateToken("user123", jwtConfig)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "Token should work when blacklist is nil")
}

func TestMiddleware_ValidJWTToken_NotBlacklisted_Succeeds(t *testing.T) {
	blacklist := NewTokenBlacklist()
	jwtConfig := JWTConfig{
		Secret:       "test-secret-key",
		AccessExpiry: time.Hour,
	}

	cfg := Config{
		Enabled:   true,
		JWTConfig: jwtConfig,
		Blacklist: blacklist,
	}
	router := setupTestRouter(cfg)

	// Generate a valid JWT token
	tokenString, err := GenerateToken("user123", jwtConfig)
	require.NoError(t, err)

	// Blacklist a different token
	blacklist.Add("different-jti", time.Now().Add(time.Hour))

	// Our token should still work
	req, err := http.NewRequest(http.MethodGet, "/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "Non-blacklisted token should work")
}
