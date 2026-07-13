// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"syscall"

	"github.com/VA7DBI/whisperAPI/audio"
	"github.com/VA7DBI/whisperAPI/config"
	"github.com/VA7DBI/whisperAPI/metrics"
	"github.com/VA7DBI/whisperAPI/parakeet/asr"
	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/gin-gonic/gin"
	"github.com/go-audio/wav"
	"github.com/jfreymuth/oggvorbis"
	"github.com/pion/opus" // Replace hraban/opus with pion/opus
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Time-related constants
	WhisperSampleLength  = 0.02 // Each sample is 20ms in Whisper
	NanosecondsPerSecond = 1_000_000_000
	EngineWhisper        = "whisper"
	EngineParakeet       = "parakeet"
)

// OGG format detection patterns
var (
	oggCapturePattern = []byte("OggS")
	vorbisHeader      = []byte("vorbis")
	opusHeader        = []byte("OpusHead")
)

// TranscriptionService encapsulates the whisper model and configuration.
type TranscriptionService struct {
	model                 whisper.Model
	whisperMu             sync.Mutex
	config                *config.Config
	parakeetTranscriber   *asr.Transcriber
	parakeetDisabled      bool
	parakeetDisableReason string
}

// TokenInfo represents token information.
type TokenInfo struct {
	Text        string  `json:"text"`
	Probability float64 `json:"probability"`
	StartTime   float64 `json:"start_time"`
	EndTime     float64 `json:"end_time"`
}

// SegmentInfo represents segment information.
type SegmentInfo struct {
	Text      string      `json:"text"`
	Speaker   string      `json:"speaker,omitempty"`
	Tokens    []TokenInfo `json:"tokens"`
	StartTime float64     `json:"start_time"`
	EndTime   float64     `json:"end_time"`
}

type DiarizationSpeaker struct {
	ID            string  `json:"id"`
	Duration      float64 `json:"duration_seconds"`
	SegmentCount  int     `json:"segment_count"`
	AverageEnergy float64 `json:"average_energy"`
}

type DiarizationResult struct {
	Enabled      bool                 `json:"enabled"`
	Method       string               `json:"method"`
	SpeakerCount int                  `json:"speaker_count"`
	Speakers     []DiarizationSpeaker `json:"speakers"`
}

type DiarizationOptions struct {
	Enabled          bool
	ExpectedSpeakers int
}

type diarizationSegmentFeatures struct {
	rms float64
	zcr float64
}

// TranscriptionResponse represents the transcription response.
type TranscriptionResponse struct {
	Text           string              `json:"text"`
	Engine         string              `json:"engine"`
	Model          string              `json:"model"`
	Diarization    *DiarizationResult  `json:"diarization,omitempty"`
	Segments       []SegmentInfo       `json:"segments"`
	Duration       float64             `json:"duration_seconds"`
	ProcessingTime float64             `json:"processing_time_seconds"`
	Confidence     float64             `json:"confidence"`
	MemoryUsage    MemStats            `json:"memory_usage"`
	AudioInfo      audio.AudioMetadata `json:"audio_info"` // Updated to use audio package type
	Timestamp      time.Time           `json:"timestamp"`
	ComputeTime    struct {
		CPUTime float64 `json:"cpu_time_seconds"`
		GPUTime float64 `json:"gpu_time_seconds,omitempty"`
	} `json:"compute_time"`
}

// MemStats represents memory statistics.
type MemStats struct {
	AllocatedMB   float64 `json:"allocated_mb"`
	TotalAllocMB  float64 `json:"total_alloc_mb"`
	SystemMB      float64 `json:"system_mb"`
	HeapInUseMB   float64 `json:"heap_in_use_mb"`
	StackInUseMB  float64 `json:"stack_in_use_mb"`
	GcCycles      uint32  `json:"gc_cycles"`
	GcPauseMicros uint64  `json:"gc_pause_micros"`
}

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

var supportedTranscriptionEngines = map[string]struct{}{
	EngineWhisper:  {},
	EngineParakeet: {},
}

type parakeetTranscriptionRequest struct {
	Model    string `json:"model,omitempty"`
	Language string `json:"language,omitempty"`
}

type parakeetTranscriptionResponse struct {
	Text       string        `json:"text"`
	Segments   []SegmentInfo `json:"segments"`
	Confidence *float64      `json:"confidence,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// NewTranscriptionService creates a new transcription service.
func NewTranscriptionService(cfg *config.Config) (*TranscriptionService, error) {
	model, err := whisper.New(cfg.Whisper.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load whisper model: %v", err)
	}

	parakeetDisabled := false
	parakeetDisableReason := ""
	parakeetTranscriber := (*asr.Transcriber)(nil)

	parakeetTranscriber, err = startEmbeddedParakeetIfEnabled(cfg)
	if err != nil {
		parakeetDisabled = true
		parakeetDisableReason = err.Error()
		log.Printf("warning: %s; parakeet engine disabled", err.Error())
	}

	if parakeetTranscriber == nil && strings.TrimSpace(cfg.Parakeet.Endpoint) == "" {
		parakeetDisabled = true
		if strings.TrimSpace(parakeetDisableReason) == "" {
			parakeetDisableReason = "embedded parakeet is disabled and no parakeet endpoint is configured"
		}
	}

	if strings.TrimSpace(cfg.Parakeet.Endpoint) != "" {
		if err := validateConfiguredParakeetEndpoint(cfg); err != nil {
			if parakeetTranscriber != nil {
				parakeetTranscriber.Close()
			}
			model.Close()
			return nil, err
		}
	}

	if parakeetTranscriber != nil {
		cfg.Parakeet.Endpoint = ""
	}

	return &TranscriptionService{
		model:                 model,
		config:                cfg,
		parakeetTranscriber:   parakeetTranscriber,
		parakeetDisabled:      parakeetDisabled,
		parakeetDisableReason: parakeetDisableReason,
	}, nil
}

func startEmbeddedParakeetIfEnabled(cfg *config.Config) (*asr.Transcriber, error) {
	if strings.TrimSpace(cfg.Parakeet.Endpoint) != "" {
		return nil, nil
	}

	if !cfg.Parakeet.Embedded.Enabled {
		return nil, nil
	}

	modelsDir := strings.TrimSpace(cfg.Parakeet.Embedded.ModelsDir)
	if modelsDir == "" {
		modelsDir = "models"
	}

	workers := cfg.Parakeet.Embedded.Workers
	if workers <= 0 {
		workers = 2
	}

	transcriber, err := asr.NewTranscriber(modelsDir, workers, asr.Options{
		FFmpeg: asr.FFmpegConfig{Enabled: true, Timeout: 60 * time.Second},
		Chunk:  asr.ChunkConfig{Enabled: false},
		GPU:    asr.GPUConfig{Provider: asr.ProviderCPU},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize embedded parakeet runtime: %v", err)
	}

	return transcriber, nil
}

func validateConfiguredParakeetEndpoint(cfg *config.Config) error {
	transcriptionURL, err := resolveParakeetTranscriptionEndpoint(strings.TrimSpace(cfg.Parakeet.Endpoint))
	if err != nil {
		return fmt.Errorf("invalid parakeet endpoint: %v", err)
	}

	if transcriptionURL == "" {
		return nil
	}

	timeout := time.Duration(cfg.Parakeet.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if err := probeParakeetEndpoint(transcriptionURL, timeout); err != nil {
		return fmt.Errorf("parakeet endpoint startup probe failed for %s: %v", transcriptionURL, err)
	}

	return nil
}

func probeParakeetEndpoint(endpoint string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodOptions, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Any non-5xx HTTP response confirms the endpoint is reachable at startup.
	if resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("received HTTP %d", resp.StatusCode)
	}

	return nil
}

func resolveParakeetTranscriptionEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", nil
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("endpoint must be an absolute URL")
	}

	cleanPath := strings.TrimSpace(parsed.Path)
	if cleanPath == "" || cleanPath == "/" {
		parsed.Path = path.Join("/", "v1", "audio", "transcriptions")
	} else {
		parsed.Path = path.Clean(cleanPath)
	}

	return parsed.String(), nil
}

// Close closes the transcription service.
func (s *TranscriptionService) Close() {
	if s.parakeetTranscriber != nil {
		s.parakeetTranscriber.Close()
	}

	s.model.Close()
}

func (s *TranscriptionService) ParakeetHealth() (enabled bool, mode, reason string) {
	if s.parakeetDisabled {
		reason = strings.TrimSpace(s.parakeetDisableReason)
		if reason == "" {
			reason = "parakeet engine is disabled"
		}
		return false, "disabled", reason
	}

	if s.parakeetTranscriber != nil {
		return true, "embedded", ""
	}

	if strings.TrimSpace(s.config.Parakeet.Endpoint) != "" {
		return true, "endpoint", ""
	}

	return false, "disabled", "parakeet is not configured"
}

// TranscribeHandler handles the transcription request.
//
//	@Summary		Transcribe audio to text
//	@Description	Converts audio file to text. Use engine=whisper (default) for local whisper.cpp processing, or engine=parakeet to forward the file to a Parakeet OpenAI-compatible endpoint.
//	@Tags			transcription
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			audio	formData	file					true	"Audio file to transcribe (WAV, MP3, OGG Vorbis, Opus, FLAC, AAC, or Speex format)"
//	@Param			engine	formData	string				false	"Speech-to-text engine to use: whisper (default) or parakeet" Enums(whisper,parakeet) default(whisper)
//	@Param			diarize	formData	boolean				false	"Enable basic speaker diarization (currently whisper engine only)" default(false)
//	@Param			expected_speakers	formData	integer	false	"Optional speaker hint for diarization (2-8)"
//	@Success		200		{object}	TranscriptionResponse	"Successful transcription with metadata"
//	@Failure		400		{object}	ErrorResponse			"Invalid request (missing file, file too large)"
//	@Failure		401		{object}	ErrorResponse			"Unauthorized (invalid or missing API key)"
//	@Failure		500		{object}	ErrorResponse			"Server error during processing"
//	@Security		ApiKeyAuth
//	@Router			/transcribe [post]
func (s *TranscriptionService) TranscribeHandler(c *gin.Context) {
	engine, err := parseTranscriptionEngine(c.PostForm("engine"))
	if err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", "unknown").Inc()
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Get file extension for metrics labeling
	file, err := c.FormFile("audio")
	if err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", "unknown").Inc()
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "No audio file provided"})
		return
	}

	// Check file size
	if file.Size > s.config.Audio.MaxFileSize*1024*1024 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("File too large. Maximum size is %dMB", s.config.Audio.MaxFileSize),
		})
		return
	}

	format := strings.ToLower(filepath.Ext(file.Filename))
	timer := prometheus.NewTimer(metrics.TranscriptionDuration.WithLabelValues(format))
	defer timer.ObserveDuration()

	// Save uploaded file temporarily. Keep extension so downstream format detection works.
	tmpPattern := "audio-*" + filepath.Ext(file.Filename)
	tmpFile, err := os.CreateTemp("", tmpPattern)
	if err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create temp audio file"})
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	src, err := file.Open()
	if err != nil {
		tmpFile.Close()
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to open uploaded audio file"})
		return
	}

	if _, err := io.Copy(tmpFile, src); err != nil {
		src.Close()
		tmpFile.Close()
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to save audio file"})
		return
	}

	if err := src.Close(); err != nil {
		tmpFile.Close()
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to close uploaded audio file"})
		return
	}

	if err := tmpFile.Close(); err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to close temp audio file"})
		return
	}

	// Record start time and memory stats
	startTime := time.Now()
	var memStats runtime.MemStats
	runtime.GC() // Run GC before measuring
	runtime.ReadMemStats(&memStats)
	startAlloc := memStats.Alloc
	startGC := memStats.NumGC
	startPause := memStats.PauseTotalNs

	// Get audio metadata before processing. For Speex, metadata parsing can fail
	// on some test fixtures even though decode support is intentionally unimplemented.
	audioInfo, err := s.getAudioMetadata(tmpPath)
	if err != nil {
		if format == ".spx" {
			audioInfo = audio.AudioMetadata{
				Format:       "Speex",
				Codec:        "Speex",
				OriginalSize: file.Size,
			}
		} else {
			metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to get audio metadata: %v", err)})
			return
		}
	}

	// For Parakeet we forward the original full clip. Whisper still needs PCM samples.
	duration := audioInfo.Duration
	var samples []float32
	if engine == EngineWhisper {
		samples, err = s.convertAudioToSamples(tmpPath)
		if err != nil {
			metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to convert audio: %v", err)})
			return
		}

		// Prefer exact sample-derived duration for Whisper pipeline.
		duration = float64(len(samples)) / float64(s.config.Audio.SampleRate)
	}

	// Track CPU time using rusage only
	var rusageStart, rusageEnd syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusageStart)

	diarizeRaw := strings.TrimSpace(strings.ToLower(c.PostForm("diarize")))
	diarizeEnabled, err := strconv.ParseBool(diarizeRaw)
	if diarizeRaw == "" {
		diarizeEnabled = false
		err = nil
	}
	if err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid diarize value, expected true or false"})
		return
	}

	if diarizeEnabled && engine != EngineWhisper {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "diarization is currently supported only with engine=whisper"})
		return
	}

	diarizationOptions := DiarizationOptions{Enabled: diarizeEnabled, ExpectedSpeakers: 0}
	if rawExpected := strings.TrimSpace(c.PostForm("expected_speakers")); rawExpected != "" {
		expected, parseErr := strconv.Atoi(rawExpected)
		if parseErr != nil {
			metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid expected_speakers value, expected integer in range 2-8"})
			return
		}
		if expected < 2 || expected > 8 {
			metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "expected_speakers must be between 2 and 8"})
			return
		}
		diarizationOptions.ExpectedSpeakers = expected
	}

	text, segments, confidence, diarization, err := s.transcribeWithEngine(engine, tmpPath, samples, diarizationOptions)
	if err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to process audio with %s: %v", engine, err)})
		return
	}

	// Calculate CPU time
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusageEnd)

	cpuTimeUser := time.Duration(rusageEnd.Utime.Nano() - rusageStart.Utime.Nano())
	cpuTimeSystem := time.Duration(rusageEnd.Stime.Nano() - rusageStart.Stime.Nano())
	cpuTimeTotal := cpuTimeUser + cpuTimeSystem

	// Record CPU time metrics
	metrics.CPUTime.WithLabelValues("user").Observe(cpuTimeUser.Seconds())
	metrics.CPUTime.WithLabelValues("system").Observe(cpuTimeSystem.Seconds())
	metrics.CPUTime.WithLabelValues("total").Observe(cpuTimeTotal.Seconds())

	// Calculate final memory stats
	runtime.GC() // Run GC after processing
	runtime.ReadMemStats(&memStats)

	const bytesToMB = 1024 * 1024
	allocatedDelta := int64(memStats.Alloc) - int64(startAlloc)
	if allocatedDelta < 0 {
		allocatedDelta = 0
	}

	response := TranscriptionResponse{
		Text:           text,
		Engine:         engine,
		Model:          s.modelNameForEngine(engine),
		Diarization:    diarization,
		Segments:       segments,
		Duration:       duration,
		ProcessingTime: time.Since(startTime).Seconds(),
		Confidence:     confidence,
		AudioInfo:      audioInfo,
		MemoryUsage: MemStats{
			AllocatedMB:   float64(allocatedDelta) / bytesToMB,
			TotalAllocMB:  float64(memStats.TotalAlloc) / bytesToMB,
			SystemMB:      float64(memStats.Sys) / bytesToMB,
			HeapInUseMB:   float64(memStats.HeapInuse) / bytesToMB,
			StackInUseMB:  float64(memStats.StackInuse) / bytesToMB,
			GcCycles:      memStats.NumGC - startGC,
			GcPauseMicros: (memStats.PauseTotalNs - startPause) / 1000,
		},
		Timestamp: time.Now(),
		ComputeTime: struct {
			CPUTime float64 `json:"cpu_time_seconds"`
			GPUTime float64 `json:"gpu_time_seconds,omitempty"`
		}{
			CPUTime: cpuTimeTotal.Seconds(),
			// GPU time would be added here if available from the model
		},
	}

	// Record memory metrics
	metrics.MemoryUsage.WithLabelValues("allocated").Set(float64(memStats.Alloc))
	metrics.MemoryUsage.WithLabelValues("system").Set(float64(memStats.Sys))
	metrics.MemoryUsage.WithLabelValues("heap").Set(float64(memStats.HeapInuse))

	// Record audio duration
	metrics.AudioDuration.WithLabelValues(format).Observe(duration)

	// Record request success
	metrics.TranscriptionRequests.WithLabelValues("success", format).Inc()

	c.JSON(http.StatusOK, response)
}

func (s *TranscriptionService) modelNameForEngine(engine string) string {
	switch engine {
	case EngineWhisper:
		return s.config.Whisper.ModelPath
	case EngineParakeet:
		if model := strings.TrimSpace(s.config.Parakeet.Model); model != "" {
			return model
		}
		return "parakeet-tdt-0.6b"
	default:
		return ""
	}
}

func parseTranscriptionEngine(raw string) (string, error) {
	engine := strings.TrimSpace(strings.ToLower(raw))
	if engine == "" {
		return EngineWhisper, nil
	}

	if _, ok := supportedTranscriptionEngines[engine]; !ok {
		return "", fmt.Errorf("unsupported engine: %s (supported engines: whisper, parakeet)", engine)
	}

	return engine, nil
}

func (s *TranscriptionService) transcribeWithEngine(engine, audioPath string, samples []float32, diarizationOptions DiarizationOptions) (string, []SegmentInfo, float64, *DiarizationResult, error) {
	switch engine {
	case EngineWhisper:
		if len(samples) == 0 {
			return "", nil, 0, nil, fmt.Errorf("no audio samples available for whisper engine")
		}
		return s.transcribeWithWhisper(samples, diarizationOptions)
	case EngineParakeet:
		if s.parakeetDisabled {
			reason := strings.TrimSpace(s.parakeetDisableReason)
			if reason == "" {
				reason = "parakeet is not available"
			}
			return "", nil, 0, nil, fmt.Errorf("parakeet engine is disabled: %s", reason)
		}
		text, segments, confidence, err := s.transcribeWithParakeet(audioPath)
		return text, segments, confidence, nil, err
	default:
		return "", nil, 0, nil, fmt.Errorf("unsupported engine: %s", engine)
	}
}

func (s *TranscriptionService) transcribeWithParakeet(audioPath string) (string, []SegmentInfo, float64, error) {
	if s.parakeetTranscriber != nil {
		audioData, err := os.ReadFile(audioPath)
		if err != nil {
			return "", nil, 0, fmt.Errorf("failed to read audio for embedded parakeet: %v", err)
		}

		format := strings.TrimPrefix(strings.ToLower(filepath.Ext(audioPath)), ".")
		text, err := s.parakeetTranscriber.Transcribe(
			context.Background(),
			audioData,
			format,
			strings.TrimSpace(s.config.Parakeet.Language),
		)
		if err != nil {
			if errors.Is(err, asr.ErrUnsupportedAudio) {
				return "", nil, 0, fmt.Errorf("unsupported audio format for embedded parakeet: %v", err)
			}
			return "", nil, 0, fmt.Errorf("embedded parakeet transcription failed: %v", err)
		}

		return text, nil, 0, nil
	}

	transcriptionURL, err := resolveParakeetTranscriptionEndpoint(s.config.Parakeet.Endpoint)
	if err != nil {
		return "", nil, 0, fmt.Errorf("invalid parakeet endpoint: %v", err)
	}

	if transcriptionURL == "" {
		return "", nil, 0, fmt.Errorf("parakeet endpoint is not configured")
	}

	audioFile, err := os.Open(audioPath)
	if err != nil {
		return "", nil, 0, fmt.Errorf("failed to read audio for parakeet: %v", err)
	}
	defer audioFile.Close()

	request := parakeetTranscriptionRequest{
		Model:    strings.TrimSpace(s.config.Parakeet.Model),
		Language: strings.TrimSpace(s.config.Parakeet.Language),
	}
	if request.Model == "" {
		request.Model = "parakeet-tdt-0.6b"
	}

	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)

	filePart, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", nil, 0, fmt.Errorf("failed to create multipart file part: %v", err)
	}

	if _, err := io.Copy(filePart, audioFile); err != nil {
		return "", nil, 0, fmt.Errorf("failed to write multipart file part: %v", err)
	}

	if err := writer.WriteField("model", request.Model); err != nil {
		return "", nil, 0, fmt.Errorf("failed to write model field: %v", err)
	}

	if request.Language != "" {
		if err := writer.WriteField("language", request.Language); err != nil {
			return "", nil, 0, fmt.Errorf("failed to write language field: %v", err)
		}
	}

	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return "", nil, 0, fmt.Errorf("failed to write response_format field: %v", err)
	}

	if err := writer.Close(); err != nil {
		return "", nil, 0, fmt.Errorf("failed to finalize multipart request: %v", err)
	}

	timeout := time.Duration(s.config.Parakeet.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	resp, err := postParakeetMultipart(transcriptionURL, writer.FormDataContentType(), multipartBody.Bytes(), timeout)
	if err != nil {
		return "", nil, 0, fmt.Errorf("failed to call parakeet endpoint: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, 0, fmt.Errorf("failed to read parakeet response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", nil, 0, fmt.Errorf("parakeet endpoint returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var parsed parakeetTranscriptionResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", nil, 0, fmt.Errorf("failed to decode parakeet response: %v", err)
	}

	if parsed.Error != "" {
		return "", nil, 0, fmt.Errorf("parakeet error: %s", parsed.Error)
	}

	confidence := 0.0
	if parsed.Confidence != nil {
		confidence = *parsed.Confidence
	}

	return parsed.Text, parsed.Segments, confidence, nil
}

func postParakeetMultipart(endpoint, contentType string, payload []byte, timeout time.Duration) (*http.Response, error) {
	client := &http.Client{Timeout: timeout}
	body := bytes.NewReader(payload)
	req, err := http.NewRequest(http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(payload))
	return client.Do(req)
}

func (s *TranscriptionService) transcribeWithWhisper(samples []float32, diarizationOptions DiarizationOptions) (string, []SegmentInfo, float64, *DiarizationResult, error) {
	// whisper.cpp context/state scheduling is not safe under concurrent calls
	// with the current binding usage, so serialize Whisper requests.
	s.whisperMu.Lock()
	defer s.whisperMu.Unlock()

	context, err := s.model.NewContext()
	if err != nil {
		return "", nil, 0, nil, fmt.Errorf("failed to create whisper context: %v", err)
	}

	language := strings.TrimSpace(s.config.Whisper.Language)
	if language != "" {
		if err := context.SetLanguage(language); err != nil {
			return "", nil, 0, nil, fmt.Errorf("failed to set whisper language %q: %v", language, err)
		}
	}

	text := ""
	var totalProb float64
	var tokenCount int
	segments := make([]SegmentInfo, 0)

	if err := context.Process(samples, nil, nil); err != nil {
		return "", nil, 0, nil, err
	}

	for {
		seg, err := context.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, 0, nil, err
		}

		text += seg.Text

		segInfo := SegmentInfo{
			Text:      seg.Text,
			StartTime: durationToSeconds(seg.Start),
			EndTime:   durationToSeconds(seg.End),
			Tokens:    make([]TokenInfo, 0, len(seg.Tokens)),
		}

		for _, token := range seg.Tokens {
			tokenInfo := TokenInfo{
				Text:        token.Text,
				Probability: float64(token.P),
				StartTime:   durationToSeconds(token.Start),
				EndTime:     durationToSeconds(token.End),
			}
			segInfo.Tokens = append(segInfo.Tokens, tokenInfo)

			totalProb += float64(token.P)
			tokenCount++
		}

		segments = append(segments, segInfo)
	}

	confidence := 0.0
	if tokenCount > 0 {
		confidence = totalProb / float64(tokenCount)
	}

	diarizationResult, segments := diarizeWhisperSegments(samples, segments, s.config.Audio.SampleRate, diarizationOptions)

	return text, segments, confidence, diarizationResult, nil
}

func diarizeWhisperSegments(samples []float32, segments []SegmentInfo, sampleRate int, options DiarizationOptions) (*DiarizationResult, []SegmentInfo) {
	if !options.Enabled || len(segments) == 0 || len(samples) == 0 || sampleRate <= 0 {
		return nil, segments
	}

	features := make([]diarizationSegmentFeatures, len(segments))
	validCount := 0
	for i, seg := range segments {
		start := int(seg.StartTime * float64(sampleRate))
		end := int(seg.EndTime * float64(sampleRate))
		if start < 0 {
			start = 0
		}
		if end > len(samples) {
			end = len(samples)
		}
		if end <= start {
			continue
		}

		rms, zcr := segmentAcousticFeatures(samples[start:end])
		features[i] = diarizationSegmentFeatures{rms: rms, zcr: zcr}
		validCount++
	}

	if validCount == 0 {
		return &DiarizationResult{Enabled: true, Method: "acoustic-kmeans", SpeakerCount: 1, Speakers: []DiarizationSpeaker{{ID: "SPEAKER_00", Duration: totalSegmentDuration(segments), SegmentCount: len(segments), AverageEnergy: 0}}}, assignSingleSpeaker(segments, "SPEAKER_00")
	}

	k := options.ExpectedSpeakers
	if k < 2 || k > 8 {
		if len(segments) < 4 {
			k = 1
		} else {
			k = 2
		}
	}
	if k > len(segments) {
		k = len(segments)
	}
	if k <= 1 {
		speakerID := "SPEAKER_00"
		for i := range segments {
			segments[i].Speaker = speakerID
		}
		return &DiarizationResult{Enabled: true, Method: "acoustic-kmeans", SpeakerCount: 1, Speakers: []DiarizationSpeaker{{ID: speakerID, Duration: totalSegmentDuration(segments), SegmentCount: len(segments), AverageEnergy: averageRMS(features)}}}, segments
	}

	points := make([][2]float64, len(segments))
	for i := range segments {
		points[i] = [2]float64{features[i].rms, features[i].zcr}
	}

	labels, centroids := kmeans2D(points, k, 16)
	if len(labels) != len(segments) {
		return &DiarizationResult{Enabled: true, Method: "acoustic-kmeans", SpeakerCount: 1, Speakers: []DiarizationSpeaker{{ID: "SPEAKER_00", Duration: totalSegmentDuration(segments), SegmentCount: len(segments), AverageEnergy: averageRMS(features)}}}, assignSingleSpeaker(segments, "SPEAKER_00")
	}

	clusterOrder := make([]int, 0, len(centroids))
	for idx := range centroids {
		clusterOrder = append(clusterOrder, idx)
	}
	sortClustersByEnergy(clusterOrder, centroids)

	clusterToSpeaker := make(map[int]string, len(clusterOrder))
	for i, clusterID := range clusterOrder {
		clusterToSpeaker[clusterID] = fmt.Sprintf("SPEAKER_%02d", i)
	}

	speakerStats := map[string]*DiarizationSpeaker{}
	for i := range segments {
		speakerID := clusterToSpeaker[labels[i]]
		segments[i].Speaker = speakerID

		dur := segments[i].EndTime - segments[i].StartTime
		if dur < 0 {
			dur = 0
		}
		if _, ok := speakerStats[speakerID]; !ok {
			speakerStats[speakerID] = &DiarizationSpeaker{ID: speakerID}
		}
		st := speakerStats[speakerID]
		st.Duration += dur
		st.SegmentCount++
		st.AverageEnergy += features[i].rms
	}

	resultSpeakers := make([]DiarizationSpeaker, 0, len(speakerStats))
	for _, clusterID := range clusterOrder {
		speakerID := clusterToSpeaker[clusterID]
		if st, ok := speakerStats[speakerID]; ok {
			if st.SegmentCount > 0 {
				st.AverageEnergy /= float64(st.SegmentCount)
			}
			resultSpeakers = append(resultSpeakers, *st)
		}
	}

	return &DiarizationResult{
		Enabled:      true,
		Method:       "acoustic-kmeans",
		SpeakerCount: len(resultSpeakers),
		Speakers:     resultSpeakers,
	}, segments
}

func segmentAcousticFeatures(samples []float32) (float64, float64) {
	if len(samples) == 0 {
		return 0, 0
	}

	var sqSum float64
	zeroCrossings := 0
	prev := samples[0]
	for _, s := range samples {
		v := float64(s)
		sqSum += v * v
		if (prev < 0 && s >= 0) || (prev >= 0 && s < 0) {
			zeroCrossings++
		}
		prev = s
	}

	rms := math.Sqrt(sqSum / float64(len(samples)))
	zcr := float64(zeroCrossings) / float64(len(samples))
	return rms, zcr
}

func kmeans2D(points [][2]float64, k, maxIterations int) ([]int, [][2]float64) {
	if len(points) == 0 || k <= 0 {
		return nil, nil
	}
	if k > len(points) {
		k = len(points)
	}

	centroids := make([][2]float64, k)
	for i := 0; i < k; i++ {
		centroids[i] = points[(i*len(points))/k]
	}

	labels := make([]int, len(points))
	for iter := 0; iter < maxIterations; iter++ {
		changed := false
		for i, point := range points {
			bestCluster := 0
			bestDistance := distance2D(point, centroids[0])
			for c := 1; c < k; c++ {
				d := distance2D(point, centroids[c])
				if d < bestDistance {
					bestDistance = d
					bestCluster = c
				}
			}
			if labels[i] != bestCluster {
				labels[i] = bestCluster
				changed = true
			}
		}

		newCentroids := make([][2]float64, k)
		counts := make([]int, k)
		for i, point := range points {
			cluster := labels[i]
			newCentroids[cluster][0] += point[0]
			newCentroids[cluster][1] += point[1]
			counts[cluster]++
		}

		for c := 0; c < k; c++ {
			if counts[c] == 0 {
				newCentroids[c] = centroids[c]
				continue
			}
			newCentroids[c][0] /= float64(counts[c])
			newCentroids[c][1] /= float64(counts[c])
		}

		centroids = newCentroids
		if !changed {
			break
		}
	}

	return labels, centroids
}

func distance2D(a, b [2]float64) float64 {
	d0 := a[0] - b[0]
	d1 := a[1] - b[1]
	return d0*d0 + d1*d1
}

func sortClustersByEnergy(order []int, centroids [][2]float64) {
	for i := 0; i < len(order)-1; i++ {
		for j := i + 1; j < len(order); j++ {
			if centroids[order[i]][0] > centroids[order[j]][0] {
				order[i], order[j] = order[j], order[i]
			}
		}
	}
}

func assignSingleSpeaker(segments []SegmentInfo, speaker string) []SegmentInfo {
	for i := range segments {
		segments[i].Speaker = speaker
	}
	return segments
}

func totalSegmentDuration(segments []SegmentInfo) float64 {
	total := 0.0
	for _, seg := range segments {
		dur := seg.EndTime - seg.StartTime
		if dur > 0 {
			total += dur
		}
	}
	return total
}

func averageRMS(features []diarizationSegmentFeatures) float64 {
	if len(features) == 0 {
		return 0
	}
	total := 0.0
	for _, f := range features {
		total += f.rms
	}
	return total / float64(len(features))
}

// handleError adds error metrics in error handlers.
func (s *TranscriptionService) handleError(c *gin.Context, format, status string, err error) {
	metrics.TranscriptionRequests.WithLabelValues(status, format).Inc()
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
}

// convertAudioToSamples converts the audio file to samples using the appropriate format handler.
func (s *TranscriptionService) convertAudioToSamples(filename string) ([]float32, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %v", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filename))
	var format audio.Format

	switch ext {
	case ".wav":
		format = &audio.WAVFormat{}
	case ".mp3":
		format = &audio.MP3Format{} // Add MP3 format
	case ".flac":
		format = &audio.FLACFormat{} // Add FLAC format
	case ".aac", ".m4a":
		format = &audio.AACFormat{} // Add AAC format
	case ".spx":
		format = &audio.SpeexFormat{} // Add Speex format
	case ".ogg":
		// Detect codec first
		codec, err := detectOggCodec(file)
		if err != nil {
			return nil, fmt.Errorf("failed to detect OGG codec: %v", err)
		}

		// Reset file position
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}

		switch codec {
		case "Vorbis":
			format = &audio.VorbisFormat{}
		case "Opus":
			format = &audio.OpusFormat{}
		default:
			return nil, fmt.Errorf("unsupported OGG codec: %s", codec)
		}
	case ".opus":
		format = &audio.OpusFormat{}
	default:
		return nil, fmt.Errorf("unsupported audio format: %s", ext)
	}

	return format.ConvertToSamples(filename, s.config.Audio.SampleRate)
}

// getAudioMetadata retrieves audio metadata using the appropriate format handler.
func (s *TranscriptionService) getAudioMetadata(filename string) (audio.AudioMetadata, error) {
	file, err := os.Open(filename)
	if err != nil {
		return audio.AudioMetadata{}, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return audio.AudioMetadata{}, err
	}

	ext := strings.ToLower(filepath.Ext(filename))
	var format audio.Format

	switch ext {
	case ".wav":
		format = &audio.WAVFormat{}
	case ".mp3":
		format = &audio.MP3Format{} // Add MP3 format
	case ".flac":
		format = &audio.FLACFormat{} // Add FLAC format
	case ".aac", ".m4a":
		format = &audio.AACFormat{} // Add AAC format
	case ".spx":
		format = &audio.SpeexFormat{} // Add Speex format
	case ".ogg":
		codec, err := detectOggCodec(file)
		if err != nil {
			return audio.AudioMetadata{}, fmt.Errorf("failed to detect OGG codec: %v", err)
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return audio.AudioMetadata{}, err
		}

		switch codec {
		case "Vorbis":
			format = &audio.VorbisFormat{}
		case "Opus":
			format = &audio.OpusFormat{}
		default:
			return audio.AudioMetadata{}, fmt.Errorf("unsupported OGG codec: %s", codec)
		}
	case ".opus":
		format = &audio.OpusFormat{}
	default:
		return audio.AudioMetadata{}, fmt.Errorf("unsupported format: %s", ext)
	}

	return format.GetMetadata(filename, fileInfo.Size())
}

// detectOggCodec detects the codec used in an OGG container.
func detectOggCodec(file *os.File) (string, error) {
	// Store current position to restore later
	startPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}
	defer file.Seek(startPos, io.SeekStart)

	// Read OGG page header
	header := make([]byte, 27) // Standard OGG page header size
	n, err := file.Read(header)
	if err != nil || n < 27 {
		return "", fmt.Errorf("failed to read OGG header: %v", err)
	}

	// Verify OGG capture pattern
	if !bytes.Equal(header[:4], oggCapturePattern) {
		return "", fmt.Errorf("not an OGG file (got %x, expected %x)", header[:4], oggCapturePattern)
	}

	// Get number of page segments
	numSegments := int(header[26])

	// Read segment table
	segmentTable := make([]byte, numSegments)
	n, err = file.Read(segmentTable)
	if err != nil || n < numSegments {
		return "", fmt.Errorf("failed to read segment table: %v", err)
	}

	// Calculate total data size from segment table
	var totalSize int
	for _, size := range segmentTable {
		totalSize += int(size)
	}

	// Read first page data
	data := make([]byte, totalSize)
	n, err = file.Read(data)
	if err != nil || n < totalSize {
		return "", fmt.Errorf("failed to read page data: %v", err)
	}

	// Look for codec headers in the first few bytes of data
	if len(data) > 7 && bytes.Equal(data[1:7], []byte("vorbis")) {
		return "Vorbis", nil
	}
	if len(data) > 8 && bytes.Contains(data[:8], []byte("OpusHead")) {
		return "Opus", nil
	}

	// If no codec was detected, try searching the entire first page
	if bytes.Contains(data, []byte("vorbis")) {
		return "Vorbis", nil
	}
	if bytes.Contains(data, []byte("OpusHead")) {
		return "Opus", nil
	}

	// Debug output
	fmt.Printf("OGG header: %x\n", header)
	fmt.Printf("First 32 bytes of data: %x\n", data[:min(32, len(data))])
	fmt.Printf("Searching for Vorbis header: %x\n", vorbisHeader)
	fmt.Printf("Searching for Opus header: %x\n", opusHeader)
	fmt.Printf("Total data size: %d\n", totalSize)

	return "", fmt.Errorf("unknown OGG codec (first page size: %d bytes)", totalSize)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// convertOggVorbisToSamples converts an OGG Vorbis file to a slice of float32 samples.
func (s *TranscriptionService) convertOggVorbisToSamples(file *os.File) ([]float32, error) {
	// Ensure we're at the start of the file
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to start: %v", err)
	}

	// Try to detect codec again to ensure we have a Vorbis stream
	codec, err := detectOggCodec(file)
	if err != nil {
		return nil, fmt.Errorf("failed to verify codec: %v", err)
	}
	if codec != "Vorbis" {
		return nil, fmt.Errorf("expected Vorbis codec, got %s", codec)
	}

	// Reset to start again for actual decoding
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to start: %v", err)
	}

	decoder, err := oggvorbis.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create OGG decoder: %v", err)
	}

	// Read all samples
	var samples []float32
	buffer := make([]float32, 16384) // Read in chunks
	for {
		n, err := decoder.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read OGG data: %v", err)
		}
		samples = append(samples, buffer[:n]...)
	}

	// Convert to mono if stereo
	if decoder.Channels() > 1 {
		samples = audio.ConvertToMono(samples, decoder.Channels())
	}

	// Resample to 16kHz if needed
	if decoder.SampleRate() != s.config.Audio.SampleRate {
		samples = resampleAudio(samples, decoder.SampleRate(), s.config.Audio.SampleRate)
	}

	return samples, nil
}

// convertWavToSamples converts a WAV file to a slice of float32 samples.
func (s *TranscriptionService) convertWavToSamples(file *os.File) ([]float32, error) {
	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}

	// Get the format before reading the buffer
	format := decoder.Format()

	// Read audio buffer
	buf, err := decoder.FullPCMBuffer()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read PCM buffer: %v", err)
	}

	// Convert int buffer to float32 samples
	numSamples := len(buf.Data)
	samples := make([]float32, numSamples)

	// Scale factor for 16-bit audio
	scale := float32(1.0 / 32768.0)

	for i, sample := range buf.Data {
		samples[i] = float32(sample) * scale
	}

	// Convert to target sample rate if needed
	if format.SampleRate != s.config.Audio.SampleRate {
		samples = resampleAudio(samples, format.SampleRate, s.config.Audio.SampleRate)
	}

	return samples, nil
}

// convertOpusToSamples converts an Opus file to a slice of float32 samples.
func (s *TranscriptionService) convertOpusToSamples(file *os.File) ([]float32, error) {
	// Start from beginning of file
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to start: %v", err)
	}

	// Create Opus decoder
	decoder := opus.NewDecoder()

	var pcm []float32
	const frameSize = 960 // 20ms at 48kHz

	var headerRead bool
	var streamStarted bool

	// Read OGG pages and extract Opus packets
	for {
		// Read OGG page header
		header := make([]byte, 27)
		n, err := file.Read(header)
		if err == io.EOF {
			break
		}
		if err != nil || n < 27 {
			return nil, fmt.Errorf("failed to read OGG page header: %v", err)
		}

		// Verify OGG capture pattern
		if !bytes.Equal(header[:4], []byte("OggS")) {
			return nil, fmt.Errorf("invalid OGG page header")
		}

		// Get number of segments
		numSegments := int(header[26])

		// Read segment table
		segmentTable := make([]byte, numSegments)
		if _, err := io.ReadFull(file, segmentTable); err != nil {
			return nil, fmt.Errorf("failed to read segment table: %v", err)
		}

		// Handle header packets
		if !headerRead {
			// Skip OpusHead and OpusTags packets
			size := 0
			for _, s := range segmentTable {
				size += int(s)
			}
			if _, err := file.Seek(int64(size), io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("failed to skip header: %v", err)
			}
			headerRead = true
			continue
		}

		// Process data packets
		for i, size := range segmentTable {
			if size == 0 {
				continue
			}

			// Read Opus packet
			packet := make([]byte, size)
			if _, err := io.ReadFull(file, packet); err != nil {
				return nil, fmt.Errorf("failed to read Opus packet: %v", err)
			}

			// Skip the first data packet if we haven't seen a header
			if !streamStarted {
				streamStarted = true
				continue
			}

			// Create output buffer for decoded PCM data
			outputPCM := make([]byte, frameSize*2) // 16-bit samples = 2 bytes per sample

			// Decode Opus frame
			_, bandwidth, err := decoder.Decode(packet, outputPCM)
			if err != nil {
				fmt.Printf("Warning: failed to decode frame %d: %v (bandwidth: %v)\n", i, err, bandwidth)
				continue
			}

			// Convert decoded bytes to float32 samples
			frame := make([]float32, frameSize)
			for i := 0; i < frameSize; i++ {
				// Convert 16-bit PCM to float32
				sample := int16(outputPCM[i*2]) | (int16(outputPCM[i*2+1]) << 8)
				frame[i] = float32(sample) / 32768.0
			}

			// Append decoded samples
			pcm = append(pcm, frame...)
		}
	}

	if len(pcm) == 0 {
		return nil, fmt.Errorf("no valid Opus frames decoded")
	}

	// Resample to target sample rate
	return resampleAudio(pcm, 48000, s.config.Audio.SampleRate), nil
}

// resampleAudio resamples audio samples from one sample rate to another using linear interpolation.
// For production, consider using a better resampling algorithm.
func resampleAudio(samples []float32, srcRate, dstRate int) []float32 {
	ratio := float64(srcRate) / float64(dstRate)
	outLen := int(float64(len(samples)) / ratio)
	resampled := make([]float32, outLen)

	for i := range resampled {
		pos := float64(i) * ratio
		idx := int(pos)
		if idx >= len(samples)-1 {
			break
		}
		frac := float32(pos - float64(idx))
		resampled[i] = samples[idx]*(1-frac) + samples[idx+1]*frac
	}

	return resampled
}

// durationToSeconds converts a time.Duration to seconds.
func durationToSeconds(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / float64(NanosecondsPerSecond)
}
