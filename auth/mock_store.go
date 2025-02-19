// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package auth

// MockTokenStore is a mock implementation of TokenStore for testing
type MockTokenStore struct {
	tokens map[string]bool
}

func NewMockTokenStore() *MockTokenStore {
	return &MockTokenStore{
		tokens: make(map[string]bool),
	}
}

func (m *MockTokenStore) ValidateToken(token string) (bool, error) {
	valid, exists := m.tokens[token]
	return valid && exists, nil
}

func (m *MockTokenStore) CacheToken(token string) error {
	m.tokens[token] = true
	return nil
}
