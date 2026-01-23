package auth

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	ErrMissingAuthHeader = errors.New("missing authorization header")
	ErrInvalidAuthHeader = errors.New("invalid authorization header format")
	ErrInvalidToken      = errors.New("invalid or expired token")
)

// Config holds JWT middleware configuration
type Config struct {
	// Enabled determines if auth is required
	Enabled bool
	// ValidTokens is a simple list of valid tokens (for backward compatibility)
	// These are checked first before JWT validation
	ValidTokens []string
	// JWTConfig holds JWT-specific configuration
	// If JWTConfig.Secret is set, JWT validation is enabled
	JWTConfig JWTConfig
	// Blacklist holds the token blacklist for logout support
	// If set, tokens will be checked against the blacklist
	Blacklist *TokenBlacklist
}

// Middleware returns a Gin middleware that validates JWT tokens
func Middleware(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth if disabled
		if !cfg.Enabled {
			c.Next()
			return
		}

		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": ErrMissingAuthHeader.Error()})
			return
		}

		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(401, gin.H{"error": ErrInvalidAuthHeader.Error()})
			return
		}
		token := parts[1]

		// Try legacy token validation first (for backward compatibility)
		valid := false
		for _, t := range cfg.ValidTokens {
			if token == t {
				valid = true
				break
			}
		}

		// If not a legacy token and JWT is enabled, try JWT validation
		if !valid && cfg.JWTConfig.Secret != "" {
			claims, err := ValidateToken(token, cfg.JWTConfig)
			if err == nil && claims != nil {
				// Check if token is blacklisted (logout support)
				if cfg.Blacklist != nil && claims.ID != "" && cfg.Blacklist.IsBlacklisted(claims.ID) {
					c.AbortWithStatusJSON(401, gin.H{"error": "token revoked"})
					return
				}
				valid = true
				// Store claims in context for handlers to use
				c.Set("jwt_claims", claims)
				c.Set("jwt_subject", claims.Subject)
			}
		}

		if !valid {
			c.AbortWithStatusJSON(401, gin.H{"error": ErrInvalidToken.Error()})
			return
		}

		c.Next()
	}
}
