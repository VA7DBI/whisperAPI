// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package auth

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/VA7DBI/whisperAPI/config"
	"github.com/stretchr/testify/assert"
)

func setupPostgresTest(t *testing.T) (*PostgresTokenStore, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}

	cfg := &config.Config{}
	cfg.Auth.Postgres.Query = "SELECT EXISTS(SELECT 1 FROM api_tokens WHERE token = $1 AND valid_until > NOW())"

	store := &PostgresTokenStore{
		db:  db,
		cfg: cfg,
	}

	return store, mock
}

func TestPostgresTokenStore(t *testing.T) {
	store, mock := setupPostgresTest(t)
	defer store.db.Close()

	t.Run("ValidateValidToken", func(t *testing.T) {
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("valid-token").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		valid, err := store.ValidateToken("valid-token")
		assert.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("ValidateInvalidToken", func(t *testing.T) {
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("invalid-token").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		valid, err := store.ValidateToken("invalid-token")
		assert.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("DatabaseError", func(t *testing.T) {
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("error-token").
			WillReturnError(sqlmock.ErrCancelled)

		valid, err := store.ValidateToken("error-token")
		assert.Error(t, err)
		assert.False(t, valid)
	})
}
