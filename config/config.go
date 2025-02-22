// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`

	API struct {
		BasePath    string `yaml:"base_path"`
		SwaggerHost string `yaml:"swagger_host"`
	} `yaml:"api"`

	Whisper struct {
		ModelPath string `yaml:"model_path"`
		Language  string `yaml:"language"`
	} `yaml:"whisper"`

	Audio struct {
		SampleRate  int   `yaml:"sample_rate"`
		MaxDuration int   `yaml:"max_duration_seconds"`
		MaxFileSize int64 `yaml:"max_file_size_mb"`
	} `yaml:"audio"`

	Metrics struct {
		Enabled bool   `yaml:"enabled"`
		Path    string `yaml:"path"`
	} `yaml:"metrics"`

	Auth struct {
		Enabled bool     `yaml:"enabled"`
		Tokens  []string `yaml:"tokens"` // Fallback static tokens
		Redis   struct {
			Enabled  bool   `yaml:"enabled"`
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			DB       int    `yaml:"db"`
			Password string `yaml:"password"`
			KeyTTL   int    `yaml:"key_ttl"` // TTL in seconds
		} `yaml:"redis"`
		Postgres struct {
			Enabled  bool   `yaml:"enabled"`
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			User     string `yaml:"user"`
			Password string `yaml:"password"`
			DBName   string `yaml:"dbname"`
			Table    string `yaml:"table"`
			Query    string `yaml:"query"` // Parameterized query for token lookup
		} `yaml:"postgres"`
	} `yaml:"auth"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Set defaults if not specified
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.Host == "" {
		config.Server.Host = "localhost"
	}
	if config.API.BasePath == "" {
		config.API.BasePath = "/"
	}
	if config.Audio.SampleRate == 0 {
		config.Audio.SampleRate = 16000
	}
	if config.Whisper.ModelPath == "" {
		config.Whisper.ModelPath = "models/ggml-base.bin"
	}
	if config.Metrics.Path == "" {
		config.Metrics.Path = "/metrics"
	}

	return config, nil
}
