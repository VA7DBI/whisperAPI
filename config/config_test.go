// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	content := []byte(`
server:
  host: testhost
  port: 9090

api:
  base_path: /api/v1
  swagger_host: test.api.com

whisper:
  model_path: /path/to/model
  language: en

audio:
  sample_rate: 16000
  max_duration_seconds: 120
  max_file_size_mb: 10

metrics:
  enabled: true
  path: /metrics
`)

	tmpfile, err := os.CreateTemp("", "config.*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write(content)
	assert.NoError(t, err)
	tmpfile.Close()

	// Test loading configuration
	cfg, err := LoadConfig(tmpfile.Name())
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify values
	assert.Equal(t, "testhost", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "/api/v1", cfg.API.BasePath)
	assert.Equal(t, 16000, cfg.Audio.SampleRate)
	assert.Equal(t, true, cfg.Metrics.Enabled)
}

func TestDefaultValues(t *testing.T) {
	// Create minimal config
	content := []byte(`{}`)

	tmpfile, err := os.CreateTemp("", "config.*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write(content)
	assert.NoError(t, err)
	tmpfile.Close()

	// Test loading configuration
	cfg, err := LoadConfig(tmpfile.Name())
	assert.NoError(t, err)

	// Verify default values
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "/", cfg.API.BasePath)
	assert.Equal(t, 16000, cfg.Audio.SampleRate)
	assert.Equal(t, "models/ggml-base.bin", cfg.Whisper.ModelPath)
	assert.Equal(t, "/metrics", cfg.Metrics.Path)
}
