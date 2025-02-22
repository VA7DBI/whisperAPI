// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package middleware

import (
	"net/http"
	"strings"

	"fmt"

	"github.com/VA7DBI/whisperAPI/auth"
	"github.com/VA7DBI/whisperAPI/config"
	"github.com/gin-gonic/gin"
)

// Update constructor type definitions to match TokenStore interface
type storeConstructor func(*config.Config) (auth.TokenStore, error)

// AuthMiddleware handles bearer token authentication
type AuthMiddleware struct {
	cfg        *config.Config
	redisStore auth.TokenStore
	pgStore    auth.TokenStore
	// Update constructor types
	redisConstructor    storeConstructor
	postgresConstructor storeConstructor
}

// NewAuthMiddleware creates a new auth middleware instance
func NewAuthMiddleware(cfg *config.Config) (*AuthMiddleware, error) {
	// Create wrapper functions to convert specific types to TokenStore interface
	redisConstructor := func(cfg *config.Config) (auth.TokenStore, error) {
		return auth.NewRedisTokenStore(cfg)
	}

	postgresConstructor := func(cfg *config.Config) (auth.TokenStore, error) {
		return auth.NewPostgresTokenStore(cfg)
	}

	m := &AuthMiddleware{
		cfg:                 cfg,
		redisConstructor:    redisConstructor,
		postgresConstructor: postgresConstructor,
	}
	return m, m.initialize()
}

func (m *AuthMiddleware) initialize() error {
	// If auth is disabled, ensure no stores are initialized
	if !m.cfg.Auth.Enabled {
		m.redisStore = nil
		m.pgStore = nil
		return nil
	}

	// Only initialize stores if auth is enabled and the respective store is enabled
	if m.cfg.Auth.Redis.Enabled {
		store, err := m.redisConstructor(m.cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize Redis store: %v", err)
		}
		m.redisStore = store
	}

	if m.cfg.Auth.Postgres.Enabled {
		store, err := m.postgresConstructor(m.cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize Postgres store: %v", err)
		}
		m.pgStore = store
	}

	return nil
}

// BearerAuthMiddleware creates a new auth middleware handler
func BearerAuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	middleware, err := NewAuthMiddleware(cfg)
	if err != nil {
		// Return a handler that always fails if setup failed
		return func(c *gin.Context) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Auth middleware setup failed"})
			c.Abort()
		}
	}
	return middleware.Handler()
}

// Handler returns the gin middleware handler function
func (m *AuthMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Fast path: if auth is disabled, allow all requests
		if !m.cfg.Auth.Enabled {
			c.Next()
			return
		}

		token := extractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Try Redis first
		if m.redisStore != nil {
			valid, err := m.redisStore.ValidateToken(token)
			if err == nil && valid {
				c.Next()
				return
			}
		}

		// Try PostgreSQL
		if m.pgStore != nil {
			valid, err := m.pgStore.ValidateToken(token)
			if err == nil && valid {
				// Cache token in Redis if found in PostgreSQL
				if m.redisStore != nil {
					_ = m.redisStore.CacheToken(token)
				}
				c.Next()
				return
			}
		}

		// Finally, check static tokens
		for _, validToken := range m.cfg.Auth.Tokens {
			if token == validToken {
				// Cache static token too
				if m.redisStore != nil {
					_ = m.redisStore.CacheToken(token)
				}
				c.Next()
				return
			}
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		c.Abort()
	}
}

func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}
