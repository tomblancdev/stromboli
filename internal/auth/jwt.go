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
	// ValidTokens is a simple list of valid tokens (for MVP)
	// In production, this should be replaced with proper JWT validation
	ValidTokens []string
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

		// Validate token
		valid := false
		for _, t := range cfg.ValidTokens {
			if token == t {
				valid = true
				break
			}
		}

		if !valid {
			c.AbortWithStatusJSON(401, gin.H{"error": ErrInvalidToken.Error()})
			return
		}

		c.Next()
	}
}
