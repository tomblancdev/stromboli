package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tomblanc/stromboli/internal/auth"
)

// TokenRequest represents a request to generate JWT tokens
type TokenRequest struct {
	ClientID string `json:"client_id" binding:"required"`
}

// TokenResponse represents a JWT token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ValidateResponse represents a token validation response
type ValidateResponse struct {
	Valid     bool   `json:"valid"`
	Subject   string `json:"subject,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

// setupAuthRoutes configures auth-specific routes
func (s *Server) setupAuthRoutes() {
	authGroup := s.router.Group("/auth")
	{
		// Generate JWT tokens (requires valid API token or credentials)
		authGroup.POST("/token", auth.Middleware(s.authConfig), s.generateToken)

		// Refresh an expiring token (public endpoint - validates refresh token internally)
		authGroup.POST("/refresh", s.refreshToken)

		// Validate a token (public endpoint - validates token internally)
		authGroup.GET("/validate", s.validateToken)
	}
}

// generateToken creates new access and refresh tokens
// @Summary Generate JWT tokens
// @Description Generate new JWT access and refresh tokens using API credentials
// @Tags auth
// @Accept json
// @Produce json
// @Param request body TokenRequest true "Token request"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /auth/token [post]
func (s *Server) generateToken(c *gin.Context) {
	// JWT must be enabled
	if s.authConfig.JWTConfig.Secret == "" {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error: "JWT authentication not configured",
		})
		return
	}

	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid request: " + err.Error(),
		})
		return
	}

	// Generate access token
	accessToken, err := auth.GenerateToken(req.ClientID, s.authConfig.JWTConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "failed to generate access token",
		})
		return
	}

	// Generate refresh token
	refreshToken, err := auth.GenerateRefreshToken(req.ClientID, s.authConfig.JWTConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "failed to generate refresh token",
		})
		return
	}

	// Calculate expiry in seconds
	expiry := s.authConfig.JWTConfig.AccessExpiry
	if expiry == 0 {
		expiry = auth.DefaultAccessExpiry
	}

	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "bearer",
		ExpiresIn:    int64(expiry.Seconds()),
	})
}

// refreshToken refreshes an access token using a refresh token
// @Summary Refresh access token
// @Description Generate a new access token using a valid refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh request"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/refresh [post]
func (s *Server) refreshToken(c *gin.Context) {
	// JWT must be enabled
	if s.authConfig.JWTConfig.Secret == "" {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error: "JWT authentication not configured",
		})
		return
	}

	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid request: " + err.Error(),
		})
		return
	}

	// Validate refresh token
	claims, err := auth.ValidateRefreshToken(req.RefreshToken, s.authConfig.JWTConfig)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "invalid or expired refresh token",
		})
		return
	}

	// Generate new access token
	accessToken, err := auth.GenerateToken(claims.Subject, s.authConfig.JWTConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "failed to generate access token",
		})
		return
	}

	// Generate new refresh token
	refreshToken, err := auth.GenerateRefreshToken(claims.Subject, s.authConfig.JWTConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "failed to generate refresh token",
		})
		return
	}

	// Calculate expiry in seconds
	expiry := s.authConfig.JWTConfig.AccessExpiry
	if expiry == 0 {
		expiry = auth.DefaultAccessExpiry
	}

	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "bearer",
		ExpiresIn:    int64(expiry.Seconds()),
	})
}

// validateToken validates a JWT token and returns its claims
// @Summary Validate JWT token
// @Description Validate a JWT token and return its claims
// @Tags auth
// @Produce json
// @Success 200 {object} ValidateResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /auth/validate [get]
func (s *Server) validateToken(c *gin.Context) {
	// JWT must be enabled
	if s.authConfig.JWTConfig.Secret == "" {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error: "JWT authentication not configured",
		})
		return
	}

	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "missing authorization header",
		})
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "invalid authorization header format",
		})
		return
	}
	token := parts[1]

	// Validate token
	claims, err := auth.ValidateToken(token, s.authConfig.JWTConfig)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "invalid or expired token",
		})
		return
	}

	c.JSON(http.StatusOK, ValidateResponse{
		Valid:     true,
		Subject:   claims.Subject,
		ExpiresAt: claims.ExpiresAt.Unix(),
	})
}
