package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomblanc/stromboli/internal/auth"
)

func setupAuthTestRouter(authCfg auth.Config) *gin.Engine {
	router := gin.New()
	s := &Server{
		router:     router,
		authConfig: authCfg,
	}
	s.setupAuthRoutes()
	return router
}

func TestGenerateTokenHandler_WithValidAPIToken_ReturnsJWTTokens(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: 24 * time.Hour,
		},
	}
	router := setupAuthTestRouter(authCfg)

	reqBody := TokenRequest{
		ClientID: "client-123",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/auth/token", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer api-token-123")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response TokenResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response.AccessToken)
	assert.NotEmpty(t, response.RefreshToken)
	assert.Equal(t, "bearer", response.TokenType)
	assert.Greater(t, response.ExpiresIn, int64(0))
}

func TestGenerateTokenHandler_WithoutAuth_Returns401(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: 24 * time.Hour,
		},
	}
	router := setupAuthTestRouter(authCfg)

	reqBody := TokenRequest{
		ClientID: "client-123",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/auth/token", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGenerateTokenHandler_WithInvalidAPIToken_Returns401(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: 24 * time.Hour,
		},
	}
	router := setupAuthTestRouter(authCfg)

	reqBody := TokenRequest{
		ClientID: "client-123",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/auth/token", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGenerateTokenHandler_WithMissingClientID_Returns400(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: 24 * time.Hour,
		},
	}
	router := setupAuthTestRouter(authCfg)

	reqBody := TokenRequest{
		ClientID: "",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/auth/token", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer api-token-123")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRefreshTokenHandler_WithValidRefreshToken_ReturnsNewTokens(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: 24 * time.Hour,
		},
	}
	router := setupAuthTestRouter(authCfg)

	// Generate a refresh token first
	refreshToken, err := auth.GenerateRefreshToken("client-123", authCfg.JWTConfig)
	require.NoError(t, err)

	reqBody := RefreshRequest{
		RefreshToken: refreshToken,
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response TokenResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response.AccessToken)
	assert.NotEmpty(t, response.RefreshToken)
	assert.Equal(t, "bearer", response.TokenType)
	assert.Greater(t, response.ExpiresIn, int64(0))
}

func TestRefreshTokenHandler_WithExpiredRefreshToken_Returns401(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: -time.Hour, // Expired
		},
	}
	router := setupAuthTestRouter(authCfg)

	// Generate an expired refresh token
	refreshToken, err := auth.GenerateRefreshToken("client-123", authCfg.JWTConfig)
	require.NoError(t, err)

	reqBody := RefreshRequest{
		RefreshToken: refreshToken,
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefreshTokenHandler_WithAccessToken_Returns401(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: 24 * time.Hour,
		},
	}
	router := setupAuthTestRouter(authCfg)

	// Try to use an access token instead of refresh token
	accessToken, err := auth.GenerateToken("client-123", authCfg.JWTConfig)
	require.NoError(t, err)

	reqBody := RefreshRequest{
		RefreshToken: accessToken,
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefreshTokenHandler_WithMissingToken_Returns400(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: 24 * time.Hour,
		},
	}
	router := setupAuthTestRouter(authCfg)

	reqBody := RefreshRequest{
		RefreshToken: "",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestValidateTokenHandler_WithValidToken_ReturnsClaimsAndSuccess(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: 24 * time.Hour,
		},
	}
	router := setupAuthTestRouter(authCfg)

	// Generate a valid access token
	accessToken, err := auth.GenerateToken("client-123", authCfg.JWTConfig)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, "/auth/validate", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response ValidateResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Valid)
	assert.Equal(t, "client-123", response.Subject)
	assert.Greater(t, response.ExpiresAt, time.Now().Unix())
}

func TestValidateTokenHandler_WithExpiredToken_Returns401(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  -time.Hour, // Expired
			RefreshExpiry: 24 * time.Hour,
		},
	}
	router := setupAuthTestRouter(authCfg)

	// Generate an expired token
	expiredToken, err := auth.GenerateToken("client-123", authCfg.JWTConfig)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, "/auth/validate", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+expiredToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestValidateTokenHandler_WithMissingToken_Returns401(t *testing.T) {
	authCfg := auth.Config{
		Enabled:     true,
		ValidTokens: []string{"api-token-123"},
		JWTConfig: auth.JWTConfig{
			Secret:        "test-jwt-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: 24 * time.Hour,
		},
	}
	router := setupAuthTestRouter(authCfg)

	req, err := http.NewRequest(http.MethodGet, "/auth/validate", nil)
	require.NoError(t, err)
	// No Authorization header

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
