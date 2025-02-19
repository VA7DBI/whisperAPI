// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package auth

import (
	"testing"
	"time"

	"github.com/VA7DBI/whisperAPI/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
)

func setupRedisTest(t *testing.T) (*RedisTokenStore, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Auth.Redis.Host = mr.Host()
	cfg.Auth.Redis.Port = mr.Server().Addr().Port
	cfg.Auth.Redis.KeyTTL = 1 // 1 second TTL for testing

	store, err := NewRedisTokenStore(cfg)
	assert.NoError(t, err)

	return store, mr
}

func TestRedisTokenStore(t *testing.T) {
	store, mr := setupRedisTest(t)
	defer mr.Close()

	t.Run("ValidateNonExistentToken", func(t *testing.T) {
		valid, err := store.ValidateToken("non-existent")
		assert.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("CacheAndValidateToken", func(t *testing.T) {
		err := store.CacheToken("test-token")
		assert.NoError(t, err)

		valid, err := store.ValidateToken("test-token")
		assert.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("TokenExpiration", func(t *testing.T) {
		err := store.CacheToken("expiring-token")
		assert.NoError(t, err)

		mr.FastForward(2 * time.Second)

		valid, err := store.ValidateToken("expiring-token")
		assert.NoError(t, err)
		assert.False(t, valid)
	})
}
