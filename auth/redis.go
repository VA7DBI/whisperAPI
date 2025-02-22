// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/VA7DBI/whisperAPI/config"
	"github.com/redis/go-redis/v9"
)

// RedisTokenStore implements TokenStore for Redis
type RedisTokenStore struct {
	client *redis.Client
	ttl    time.Duration
	cfg    *config.Config
}

func NewRedisTokenStore(cfg *config.Config) (*RedisTokenStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Auth.Redis.Host, cfg.Auth.Redis.Port),
		Password: cfg.Auth.Redis.Password,
		DB:       cfg.Auth.Redis.DB,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %v", err)
	}

	return &RedisTokenStore{
		client: client,
		ttl:    time.Duration(cfg.Auth.Redis.KeyTTL) * time.Second,
		cfg:    cfg,
	}, nil
}

func (s *RedisTokenStore) ValidateToken(token string) (bool, error) {
	ctx := context.Background()
	exists, err := s.client.Exists(ctx, token).Result()
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}

func (s *RedisTokenStore) CacheToken(token string) error {
	ctx := context.Background()
	return s.client.Set(ctx, token, "1", s.ttl).Err()
}
