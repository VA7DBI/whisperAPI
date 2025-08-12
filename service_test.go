// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VA7DBI/whisperAPI/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func setupTestServer(t *testing.T) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Create test configuration
	cfg := &config.Config{}
	cfg.Audio.SampleRate = 16000
	cfg.Audio.MaxFileSize = 25
	cfg.Whisper.ModelPath = "models/ggml-base.bin"

	service, err := NewTranscriptionService(cfg)
	assert.NoError(t, err)

	r.POST("/transcribe", service.TranscribeHandler)
	return r
}

func createTestAudioFiles(t *testing.T) (string, string, string, string, string) {
	// Create test fixtures directory if it doesn't exist
	fixturesDir := "test_fixtures"
	if err := os.MkdirAll(fixturesDir, 0755); err != nil {
		t.Fatalf("Failed to create fixtures directory: %v", err)
	}

	// Check for test files
	wavPath := filepath.Join(fixturesDir, "test.wav")
	oggPath := filepath.Join(fixturesDir, "test.ogg")
	mp3Path := filepath.Join(fixturesDir, "test.mp3")
	flacPath := filepath.Join(fixturesDir, "test.flac")
	aacPath := filepath.Join(fixturesDir, "test.aac")

	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skipf("Test WAV file not found at %s - please add test fixtures", wavPath)
	}
	if _, err := os.Stat(oggPath); os.IsNotExist(err) {
		t.Skipf("Test OGG file not found at %s - please add test fixtures", oggPath)
	}
	if _, err := os.Stat(mp3Path); os.IsNotExist(err) {
		t.Skipf("Test MP3 file not found at %s - please add test fixtures", mp3Path)
	}
	if _, err := os.Stat(flacPath); os.IsNotExist(err) {
		t.Skipf("Test FLAC file not found at %s - please add test fixtures", flacPath)
	}
	if _, err := os.Stat(aacPath); os.IsNotExist(err) {
		t.Logf("Test AAC file not found at %s - AAC tests will be skipped", aacPath)
	}

	return wavPath, oggPath, mp3Path, flacPath, aacPath
}

func TestTranscribeHandler(t *testing.T) {
	r := setupTestServer(t)
	wavPath, oggPath, mp3Path, flacPath, aacPath := createTestAudioFiles(t)

	// Test WAV file
	t.Run("WAV File", func(t *testing.T) {
		testTranscription(t, r, wavPath)
	})

	// Test OGG file
	t.Run("OGG File", func(t *testing.T) {
		testTranscription(t, r, oggPath)
	})

	// Test MP3 file
	t.Run("MP3 File", func(t *testing.T) {
		testTranscription(t, r, mp3Path)
	})

	// Test FLAC file
	t.Run("FLAC File", func(t *testing.T) {
		testTranscription(t, r, flacPath)
	})

	// Test AAC file (if available)
	if _, err := os.Stat(aacPath); err == nil {
		t.Run("AAC File", func(t *testing.T) {
			testTranscriptionWithExpectedError(t, r, aacPath, "not fully implemented")
		})
	}
}

func testTranscription(t *testing.T, r *gin.Engine, audioPath string) {
	// Create a multipart form with the audio file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	file, err := os.Open(audioPath)
	assert.NoError(t, err)
	defer file.Close()

	part, err := writer.CreateFormFile("audio", filepath.Base(audioPath))
	assert.NoError(t, err)

	_, err = io.Copy(part, file)
	assert.NoError(t, err)

	writer.Close()

	// Create request
	req := httptest.NewRequest("POST", "/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Perform request
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Print the raw JSON response
	t.Logf("Response JSON for %s: %s", filepath.Base(audioPath), w.Body.String())

	// Assert response code
	assert.Equal(t, http.StatusOK, w.Code)

	var response TranscriptionResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Detailed assertions for the response
	t.Logf("Transcribed Text: %q", response.Text)
	assert.NotEmpty(t, response.Text, "Transcription text should not be empty")

	// Performance metrics logging
	t.Logf("Processing Time: %.2f seconds", response.ProcessingTime)
	t.Logf("Audio Duration: %.2f seconds", response.Duration)
	t.Logf("Memory Usage: %.2f MB", response.MemoryUsage.AllocatedMB)

	// Add assertions for the response
	assert.NotEmpty(t, response.Text)
	assert.Greater(t, response.Duration, float64(0))
	assert.Greater(t, response.ProcessingTime, float64(0))
	assert.Greater(t, response.MemoryUsage.AllocatedMB, float64(0))
}

func testTranscriptionWithExpectedError(t *testing.T, r *gin.Engine, audioPath string, expectedError string) {
	// Create a multipart form with the audio file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	file, err := os.Open(audioPath)
	assert.NoError(t, err)
	defer file.Close()

	part, err := writer.CreateFormFile("audio", filepath.Base(audioPath))
	assert.NoError(t, err)

	_, err = io.Copy(part, file)
	assert.NoError(t, err)

	writer.Close()

	// Create request
	req := httptest.NewRequest("POST", "/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Perform request
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Print the raw JSON response
	t.Logf("Response JSON for %s: %s", filepath.Base(audioPath), w.Body.String())

	// Assert that we get an error response (not 200)
	assert.NotEqual(t, http.StatusOK, w.Code)
	
	// Check that the error message contains the expected error
	assert.Contains(t, w.Body.String(), expectedError)
}

func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", healthCheck)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestSwaggerEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Test Swagger JSON endpoint
	req := httptest.NewRequest("GET", "/swagger/doc.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, strings.Contains(w.Body.String(), "Whisper API Service"))

	// Validate Swagger JSON structure
	var swaggerDoc map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &swaggerDoc)
	assert.NoError(t, err)

	// Check required Swagger fields
	assert.NotEmpty(t, swaggerDoc["swagger"])
	assert.NotEmpty(t, swaggerDoc["info"])
	assert.NotEmpty(t, swaggerDoc["paths"])

	// Test Swagger UI endpoint
	req = httptest.NewRequest("GET", "/swagger/index.html", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, strings.Contains(w.Body.String(), "swagger-ui"))
}

// Add test for API documentation completeness
func TestAPIDocumentation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	req := httptest.NewRequest("GET", "/swagger/doc.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var swaggerDoc map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &swaggerDoc)
	assert.NoError(t, err)

	paths := swaggerDoc["paths"].(map[string]interface{})

	// Check transcribe endpoint documentation
	transcribePath := paths["/transcribe"].(map[string]interface{})
	assert.NotNil(t, transcribePath["post"])
	postOp := transcribePath["post"].(map[string]interface{})
	assert.NotEmpty(t, postOp["summary"])
	assert.NotEmpty(t, postOp["parameters"])
	assert.NotEmpty(t, postOp["responses"])

	// Check health endpoint documentation
	healthPath := paths["/health"].(map[string]interface{})
	assert.NotNil(t, healthPath["get"])
	getOp := healthPath["get"].(map[string]interface{})
	assert.NotEmpty(t, getOp["summary"])
	assert.NotEmpty(t, getOp["responses"])
}
