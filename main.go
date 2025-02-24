// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/VA7DBI/whisperAPI/config"
	_ "github.com/VA7DBI/whisperAPI/docs"
	"github.com/VA7DBI/whisperAPI/middleware"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var (
	configFile = flag.String("config", "config.yaml", "Path to configuration file")
)

// @title           Whisper API Service
// @version         1.1
// @description     A self-hosted voice-to-text transcription service using Whisper AI. Supports WAV, MP3, OGG (Vorbis), and Opus formats.
// @termsOfService  http://openradiomap.com/tos/
// @contact.name    Darcy Buskermolen
// @contact.email   darcy@dbitech.ca
// @license.name    BSD-3-Clause
// @license.url     https://github.com/VA7DBI/whisperAPI/blob/main/LICENSE
// @host           api.openradiomap.com
// @BasePath       /
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	r := gin.Default()

	// Initialize transcription service with config
	service, err := NewTranscriptionService(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize transcription service: %v", err)
	}
	defer service.Close()

	// Register routes with auth middleware
	authMiddleware, err := middleware.NewAuthMiddleware(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize auth middleware: %v", err)
	}
	r.POST("/transcribe", authMiddleware.Handler(), service.TranscribeHandler)

	// These endpoints remain public
	r.GET("/health", healthCheck)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Add Prometheus metrics endpoint if enabled
	if cfg.Metrics.Enabled {
		r.GET(cfg.Metrics.Path, gin.WrapH(promhttp.Handler()))
	}

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	r.Run(addr)
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status string `json:"status"`
}

// healthCheck is the health check endpoint.
// @Summary     Health check endpoint
// @Description Get API health status
// @Tags        health
// @Produce     json
// @Success     200 {object} HealthResponse
// @Router      /health [get]
func healthCheck(c *gin.Context) {
	c.JSON(200, HealthResponse{Status: "ok"})
}
