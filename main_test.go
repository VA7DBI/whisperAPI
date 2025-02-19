// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"testing"

	"github.com/VA7DBI/whisperAPI/config"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func TestMainSetup(t *testing.T) {
	// Test router setup
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Create test configuration
	cfg := &config.Config{}
	cfg.Server.Port = 8080
	cfg.Server.Host = "localhost"
	cfg.Audio.SampleRate = 16000
	cfg.Audio.MaxFileSize = 25
	cfg.Whisper.ModelPath = "models/ggml-base.bin"
	cfg.Metrics.Enabled = true
	cfg.Metrics.Path = "/metrics"

	// Initialize transcription service
	service, err := NewTranscriptionService(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, service)
	defer service.Close()

	// Register all routes
	r.POST("/transcribe", service.TranscribeHandler)
	r.GET("/health", healthCheck)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Get all registered routes
	routes := r.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		routeMap[route.Path] = true
	}

	// Verify required endpoints are registered
	assert.True(t, routeMap["/transcribe"], "Missing /transcribe endpoint")
	assert.True(t, routeMap["/health"], "Missing /health endpoint")
	assert.True(t, routeMap["/swagger/*any"], "Missing /swagger endpoint")
	assert.True(t, routeMap["/metrics"], "Missing /metrics endpoint")
}
