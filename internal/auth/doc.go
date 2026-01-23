// Package auth provides JWT-based authentication and authorization for Stromboli.
//
// # Features
//
//   - JWT token generation and validation
//   - Token refresh with refresh tokens
//   - Token blacklist for logout support
//   - Gin middleware for route protection
//   - Configurable via Config struct
//
// # Usage
//
// Create a middleware with authentication config:
//
//	cfg := auth.Config{
//	    Enabled: true,
//	    JWTConfig: auth.JWTConfig{
//	        Secret:              "your-secret",
//	        AccessTokenExpiry:   15 * time.Minute,
//	        RefreshTokenExpiry:  24 * time.Hour,
//	    },
//	}
//	router.Use(auth.Middleware(cfg))
//
// Generate tokens:
//
//	generator := auth.NewTokenGenerator(cfg.JWTConfig)
//	access, refresh, err := generator.GenerateTokenPair("user-id")
//
// Validate tokens:
//
//	claims, err := generator.ValidateToken(tokenString)
//
// # Token Blacklist
//
// For logout support, use the TokenBlacklist:
//
//	blacklist := auth.NewTokenBlacklist()
//	blacklist.StartCleanup(time.Hour)
//	blacklist.Add(claims.ID, claims.ExpiresAt.Time)
package auth
