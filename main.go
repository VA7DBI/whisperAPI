// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

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

// @title						Whisper API Service
// @version					1.1
// @description				A self-hosted voice-to-text transcription service using Whisper AI. Supports WAV, MP3, OGG (Vorbis), Opus, FLAC, AAC (.aac/.m4a), and Speex (.spx) formats.
// @termsOfService				http://openradiomap.com/tos/
// @contact.name				Darcy Buskermolen
// @contact.email				darcy@dbitech.ca
// @license.name				BSD-3-Clause
// @license.url				https://github.com/VA7DBI/whisperAPI/blob/main/LICENSE
// @host						api.openradiomap.com
// @BasePath					/
// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						X-API-Key
func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	r := gin.Default()

	if cfg.CORS.Enabled {
		r.Use(corsMiddleware(cfg))
	}

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
//
//	@Summary		Health check endpoint
//	@Description	Get API health status
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	HealthResponse
//	@Router			/health [get]
func healthCheck(c *gin.Context) {
	c.JSON(200, HealthResponse{Status: "ok"})
}

func corsMiddleware(cfg *config.Config) gin.HandlerFunc {
	allowMethods := strings.Join(cfg.CORS.AllowedMethods, ", ")
	allowHeaders := strings.Join(cfg.CORS.AllowedHeaders, ", ")
	exposeHeaders := strings.Join(cfg.CORS.ExposeHeaders, ", ")

	allowedOrigins := make(map[string]struct{}, len(cfg.CORS.AllowedOrigins))
	for _, origin := range cfg.CORS.AllowedOrigins {
		allowedOrigins[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowOrigin := cfg.CORS.AllowAllOrigins

		if !allowOrigin {
			_, allowOrigin = allowedOrigins[origin]
		}

		if allowOrigin {
			if cfg.CORS.AllowAllOrigins {
				c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				c.Writer.Header().Set("Vary", "Origin")
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", allowMethods)
		c.Writer.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		if exposeHeaders != "" {
			c.Writer.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
		}

		if c.Request.Method == "OPTIONS" {
			if !allowOrigin {
				c.AbortWithStatus(403)
				return
			}
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
