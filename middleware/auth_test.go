// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VA7DBI/whisperAPI/auth"
	"github.com/VA7DBI/whisperAPI/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// mockTokenValidator implements both RedisTokenStore and PostgresTokenStore interfaces
type mockTokenValidator struct {
	tokens map[string]bool
}

func newMockValidator() *mockTokenValidator {
	return &mockTokenValidator{
		tokens: make(map[string]bool),
	}
}

func (m *mockTokenValidator) ValidateToken(token string) (bool, error) {
	valid, exists := m.tokens[token]
	return valid && exists, nil
}

func (m *mockTokenValidator) CacheToken(token string) error {
	m.tokens[token] = true
	return nil
}

func setupAuthTest() (*config.Config, *mockTokenValidator, *mockTokenValidator) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.Tokens = []string{"static-token"}

	// Create mock stores
	mockRedis := newMockValidator()
	mockPg := newMockValidator()

	return cfg, mockRedis, mockPg
}

// Add mock store constructors
type mockStoreConstructor = storeConstructor

func mockRedisConstructor(cfg *config.Config) (auth.TokenStore, error) {
	return newMockValidator(), nil
}

func mockPostgresConstructor(cfg *config.Config) (auth.TokenStore, error) {
	return newMockValidator(), nil
}

// ...existing code...

func TestAuthMiddleware(t *testing.T) {
	cfg, mockRedis, mockPg := setupAuthTest()

	t.Run("AuthDisabled", func(t *testing.T) {
		r := gin.New()
		cfg.Auth.Enabled = false

		middleware := &AuthMiddleware{
			cfg:        cfg,
			redisStore: mockRedis,
			pgStore:    mockPg,
		}

		r.GET("/test", middleware.Handler(), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ValidStaticToken", func(t *testing.T) {
		r := gin.New()
		cfg.Auth.Enabled = true

		middleware := &AuthMiddleware{
			cfg:        cfg,
			redisStore: mockRedis,
			pgStore:    mockPg,
		}

		r.GET("/test", middleware.Handler(), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer static-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		r := gin.New() // Create new router for this test
		cfg.Auth.Enabled = true
		middleware := &AuthMiddleware{
			cfg:        cfg,
			redisStore: mockRedis,
			pgStore:    mockPg,
		}

		r.GET("/test", middleware.Handler(), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("MissingAuthHeader", func(t *testing.T) {
		r := gin.New() // Create new router for this test
		cfg.Auth.Enabled = true
		middleware := &AuthMiddleware{
			cfg:        cfg,
			redisStore: mockRedis,
			pgStore:    mockPg,
		}

		r.GET("/test", middleware.Handler(), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestTokenValidationFlow(t *testing.T) {
	cfg, mockRedis, mockPg := setupAuthTest()
	r := gin.New()

	middleware := &AuthMiddleware{
		cfg:        cfg,
		redisStore: mockRedis,
		pgStore:    mockPg,
	}

	r.GET("/test", middleware.Handler(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	t.Run("TokenFoundInRedis", func(t *testing.T) {
		mockRedis.CacheToken("redis-token")

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer redis-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("TokenFoundInPostgres", func(t *testing.T) {
		mockPg.CacheToken("pg-token")

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer pg-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify token was cached in Redis
		valid, _ := mockRedis.ValidateToken("pg-token")
		assert.True(t, valid)
	})
}

func TestAuthConfigurationBehavior(t *testing.T) {
	t.Run("AuthDisabledOverridesStores", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Auth.Enabled = false
		cfg.Auth.Redis.Enabled = true    // Should be ignored
		cfg.Auth.Postgres.Enabled = true // Should be ignored

		middleware := &AuthMiddleware{
			cfg:                 cfg,
			redisConstructor:    mockRedisConstructor,
			postgresConstructor: mockPostgresConstructor,
		}
		err := middleware.initialize()
		assert.NoError(t, err)
		assert.Nil(t, middleware.redisStore, "Redis store should be nil when auth is disabled")
		assert.Nil(t, middleware.pgStore, "Postgres store should be nil when auth is disabled")
	})

	t.Run("AuthEnabledRespectsStoreConfig", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Auth.Enabled = true

		// Test Redis only
		cfg.Auth.Redis.Enabled = true
		cfg.Auth.Postgres.Enabled = false

		middleware := &AuthMiddleware{
			cfg:                 cfg,
			redisConstructor:    mockRedisConstructor,
			postgresConstructor: mockPostgresConstructor,
		}
		err := middleware.initialize()
		assert.NoError(t, err)
		assert.NotNil(t, middleware.redisStore, "Redis store should be initialized")
		assert.Nil(t, middleware.pgStore, "Postgres store should be nil")

		// Test Postgres only
		cfg.Auth.Redis.Enabled = false
		cfg.Auth.Postgres.Enabled = true

		middleware = &AuthMiddleware{
			cfg:                 cfg,
			redisConstructor:    mockRedisConstructor,
			postgresConstructor: mockPostgresConstructor,
		}
		err = middleware.initialize()
		assert.NoError(t, err)
		assert.Nil(t, middleware.redisStore, "Redis store should be nil")
		assert.NotNil(t, middleware.pgStore, "Postgres store should be initialized")

		// Test both enabled
		cfg.Auth.Redis.Enabled = true
		cfg.Auth.Postgres.Enabled = true

		middleware = &AuthMiddleware{
			cfg:                 cfg,
			redisConstructor:    mockRedisConstructor,
			postgresConstructor: mockPostgresConstructor,
		}
		err = middleware.initialize()
		assert.NoError(t, err)
		assert.NotNil(t, middleware.redisStore, "Redis store should be initialized")
		assert.NotNil(t, middleware.pgStore, "Postgres store should be initialized")
	})
}
