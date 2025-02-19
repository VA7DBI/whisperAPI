// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package auth

import (
	"database/sql"
	"fmt"

	"github.com/VA7DBI/whisperAPI/config"
	_ "github.com/lib/pq"
)

// PostgresTokenStore implements TokenStore for PostgreSQL
type PostgresTokenStore struct {
	db  *sql.DB
	cfg *config.Config
}

func NewPostgresTokenStore(cfg *config.Config) (*PostgresTokenStore, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Auth.Postgres.Host,
		cfg.Auth.Postgres.Port,
		cfg.Auth.Postgres.User,
		cfg.Auth.Postgres.Password,
		cfg.Auth.Postgres.DBName,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("postgres connection failed: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("postgres ping failed: %v", err)
	}

	return &PostgresTokenStore{
		db:  db,
		cfg: cfg,
	}, nil
}

func (s *PostgresTokenStore) ValidateToken(token string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(s.cfg.Auth.Postgres.Query, token).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists, nil
}

// CacheToken is a no-op for PostgreSQL as it doesn't need caching
func (s *PostgresTokenStore) CacheToken(token string) error {
	// PostgreSQL doesn't need to cache tokens
	return nil
}

func (s *PostgresTokenStore) Close() error {
	return s.db.Close()
}
