// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package auth

// TokenStore defines the basic token operations
type TokenStore interface {
	ValidateToken(token string) (bool, error)
	CacheToken(token string) error
}

type TokenInfo struct {
	Token      string
	ValidUntil int64
	UserID     string
	Scopes     []string
}
