// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenStoreInterface(t *testing.T) {
	var store TokenStore = NewMockTokenStore()

	// Test token validation
	valid, err := store.ValidateToken("test-token")
	assert.NoError(t, err)
	assert.False(t, valid)

	// Test token caching
	err = store.CacheToken("test-token")
	assert.NoError(t, err)

	// Test cached token validation
	valid, err = store.ValidateToken("test-token")
	assert.NoError(t, err)
	assert.True(t, valid)
}
