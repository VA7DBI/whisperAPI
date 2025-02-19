// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"syscall"

	"github.com/VA7DBI/whisperAPI/config"
	"github.com/VA7DBI/whisperAPI/metrics"
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
)

type TranscriptionService struct {
	model  whisper.Model
	config *config.Config
}

// Add new types for token information
type TokenInfo struct {
	Text        string  `json:"text"`
	Probability float64 `json:"probability"`
	StartTime   float64 `json:"start_time"`
	EndTime     float64 `json:"end_time"`
}

type SegmentInfo struct {
	Text      string      `json:"text"`
	Tokens    []TokenInfo `json:"tokens"`
	StartTime float64     `json:"start_time"`
	EndTime   float64     `json:"end_time"`
}

// Add new types for audio metadata
type AudioMetadata struct {
	Format       string  `json:"format"`
	Codec        string  `json:"codec"`
	SampleRate   int     `json:"sample_rate"`
	Channels     int     `json:"channels"`
	BitDepth     int     `json:"bit_depth"`
	Duration     float64 `json:"duration_seconds"`
	OriginalSize int64   `json:"original_size_bytes"`
	Bitrate      int     `json:"bitrate_kbps,omitempty"`
}

type TranscriptionResponse struct {
	Text           string        `json:"text"`
	Segments       []SegmentInfo `json:"segments"`
	Duration       float64       `json:"duration_seconds"`
	ProcessingTime float64       `json:"processing_time_seconds"`
	Confidence     float64       `json:"confidence"`
	MemoryUsage    MemStats      `json:"memory_usage"`
	AudioInfo      AudioMetadata `json:"audio_info"`
	Timestamp      time.Time     `json:"timestamp"`
	ComputeTime    struct {
		CPUTime float64 `json:"cpu_time_seconds"`
		GPUTime float64 `json:"gpu_time_seconds,omitempty"`
	} `json:"compute_time"`
}

type MemStats struct {
	AllocatedMB   float64 `json:"allocated_mb"`
	TotalAllocMB  float64 `json:"total_alloc_mb"`
	SystemMB      float64 `json:"system_mb"`
	HeapInUseMB   float64 `json:"heap_in_use_mb"`
	StackInUseMB  float64 `json:"stack_in_use_mb"`
	GcCycles      uint32  `json:"gc_cycles"`
	GcPauseMicros uint64  `json:"gc_pause_micros"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error string `json:"error"`
}

func NewTranscriptionService(cfg *config.Config) (*TranscriptionService, error) {
	model, err := whisper.New(cfg.Whisper.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load whisper model: %v", err)
	}

	return &TranscriptionService{
		model:  model,
		config: cfg,
	}, nil
}

func (s *TranscriptionService) Close() {
	s.model.Close()
}

// @Summary     Transcribe audio to text
// @Description Convert audio file to text using Whisper AI
// @Tags        transcription
// @Accept      multipart/form-data
// @Produce     json
// @Param       audio formData file true "Audio file to transcribe (WAV or OGG format)"
// @Success     200 {object} TranscriptionResponse
// @Failure     400 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /transcribe [post]
func (s *TranscriptionService) TranscribeHandler(c *gin.Context) {
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

	// Save uploaded file temporarily
	tmpFile, err := os.CreateTemp("", "audio-*"+file.Filename)
	if err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to save audio file"})
		return
	}
	defer os.Remove(tmpFile.Name())

	if err := c.SaveUploadedFile(file, tmpFile.Name()); err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to save audio file"})
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

	// Get audio metadata before processing
	audioInfo, err := s.getAudioMetadata(tmpFile.Name())
	if err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to get audio metadata: %v", err)})
		return
	}

	// Process the audio file
	context, err := s.model.NewContext()
	if err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create whisper context"})
		return
	}

	// Set up callbacks for collecting segments
	text := ""
	var totalProb float64
	var tokenCount int
	var segments []SegmentInfo

	// Convert the audio file to samples (implementation needed)
	samples, err := s.convertAudioToSamples(tmpFile.Name())
	if err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to convert audio: %v", err)})
		return
	}

	// Calculate actual duration from samples
	duration := float64(len(samples)) / float64(s.config.Audio.SampleRate)

	// Process using callbacks with correct types
	segmentCallback := func(seg whisper.Segment) {
		text += seg.Text

		// Create segment info with tokens
		segInfo := SegmentInfo{
			Text:      seg.Text,
			StartTime: durationToSeconds(seg.Start),
			EndTime:   durationToSeconds(seg.End),
			Tokens:    make([]TokenInfo, 0, len(seg.Tokens)),
		}

		// Collect token information
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

	// Track CPU time using rusage only
	var rusageStart, rusageEnd syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusageStart)

	// Process audio
	if err := context.Process(samples, segmentCallback, nil); err != nil {
		metrics.TranscriptionRequests.WithLabelValues("error", format).Inc()
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to process audio: %v", err)})
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

	// Calculate average confidence across all tokens
	confidence := 0.0
	if tokenCount > 0 {
		confidence = totalProb / float64(tokenCount)
	}

	// Calculate final memory stats
	runtime.GC() // Run GC after processing
	runtime.ReadMemStats(&memStats)

	const bytesToMB = 1024 * 1024

	response := TranscriptionResponse{
		Text:           text,
		Segments:       segments,
		Duration:       duration,
		ProcessingTime: time.Since(startTime).Seconds(),
		Confidence:     confidence,
		AudioInfo:      audioInfo,
		MemoryUsage: MemStats{
			AllocatedMB:   float64(memStats.Alloc-startAlloc) / bytesToMB,
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

// Add error metrics in error handlers
func (s *TranscriptionService) handleError(c *gin.Context, format, status string, err error) {
	metrics.TranscriptionRequests.WithLabelValues(status, format).Inc()
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
}

// Add helper function to convert audio file to samples
func (s *TranscriptionService) convertAudioToSamples(filename string) ([]float32, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %v", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".wav":
		return s.convertWavToSamples(file)
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
			return s.convertOggVorbisToSamples(file)
		case "Opus":
			return s.convertOpusToSamples(file)
		default:
			return nil, fmt.Errorf("unsupported OGG codec: %s", codec)
		}
	case ".opus":
		return s.convertOpusToSamples(file)
	default:
		return nil, fmt.Errorf("unsupported audio format: %s", ext)
	}
}

// Update magic headers with more precise patterns
var (
	// Update magic headers for more precise patterns
	// OGG capture pattern: "OggS"
	oggCapturePattern = []byte("OggS")
	// Vorbis identification header begins with \x01vorbis
	vorbisHeader = []byte{0x01, 0x76, 0x6f, 0x72, 0x62, 0x69, 0x73}
	// Opus identification header begins with "OpusHead"
	opusHeader = []byte("OpusHead")
)

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

// Rename existing convertOggToSamples to convertOggVorbisToSamples
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
		samples = convertToMono(samples, decoder.Channels())
	}

	// Resample to 16kHz if needed
	if decoder.SampleRate() != s.config.Audio.SampleRate {
		samples = resampleAudio(samples, decoder.SampleRate(), s.config.Audio.SampleRate)
	}

	return samples, nil
}

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

func convertToMono(samples []float32, channels int) []float32 {
	monoSamples := make([]float32, len(samples)/channels)
	for i := 0; i < len(monoSamples); i++ {
		sum := float32(0)
		for ch := 0; ch < channels; ch++ {
			sum += samples[i*channels+ch]
		}
		monoSamples[i] = sum / float32(channels)
	}
	return monoSamples
}

// Simple linear resampling - for production, consider using a better resampling algorithm
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

// Helper function to convert time.Duration to seconds
func durationToSeconds(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / float64(NanosecondsPerSecond)
}

func (s *TranscriptionService) getAudioMetadata(filename string) (AudioMetadata, error) {
	file, err := os.Open(filename)
	if err != nil {
		return AudioMetadata{}, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return AudioMetadata{}, err
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".wav":
		return s.getWavMetadata(file, fileInfo.Size())
	case ".ogg":
		// Detect codec first
		codec, err := detectOggCodec(file)
		if err != nil {
			return AudioMetadata{}, fmt.Errorf("failed to detect OGG codec: %v", err)
		}

		// Reset file position
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return AudioMetadata{}, err
		}

		switch codec {
		case "Vorbis":
			metadata, err := s.getOggVorbisMetadata(file, fileInfo.Size())
			metadata.Format = "OGG" // Container format
			return metadata, err
		case "Opus":
			metadata, err := s.getOpusMetadata(file, fileInfo.Size())
			metadata.Format = "OGG" // Container format
			return metadata, err
		default:
			return AudioMetadata{}, fmt.Errorf("unsupported OGG codec: %s", codec)
		}
	case ".opus":
		return s.getOpusMetadata(file, fileInfo.Size())
	default:
		return AudioMetadata{}, fmt.Errorf("unsupported format: %s", ext)
	}
}

// Rename existing getOggMetadata to getOggVorbisMetadata
func (s *TranscriptionService) getOggVorbisMetadata(file *os.File, size int64) (AudioMetadata, error) {
	decoder, err := oggvorbis.NewReader(file)
	if err != nil {
		return AudioMetadata{}, err
	}

	// Get duration by seeking to end
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return AudioMetadata{}, err
	}

	maxPos, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return AudioMetadata{}, err
	}

	_, err = file.Seek(currentPos, io.SeekStart)
	if err != nil {
		return AudioMetadata{}, err
	}

	duration := float64(maxPos) / float64(decoder.SampleRate()*decoder.Channels()*2)

	return AudioMetadata{
		Format:       "OGG",
		Codec:        "Vorbis",
		SampleRate:   decoder.SampleRate(),
		Channels:     decoder.Channels(),
		BitDepth:     16, // Vorbis typically uses 16-bit samples
		Duration:     duration,
		OriginalSize: size,
		Bitrate:      int(float64(size*8) / duration / 1000),
	}, nil
}

func (s *TranscriptionService) getWavMetadata(file *os.File, size int64) (AudioMetadata, error) {
	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return AudioMetadata{}, fmt.Errorf("invalid WAV file")
	}

	format := decoder.Format()
	dur, err := decoder.Duration()
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to get duration: %v", err)
	}

	// Calculate bitrate from file size and duration
	bitrate := int(float64(size*8) / dur.Seconds() / 1000)

	return AudioMetadata{
		Format:       "WAV",
		Codec:        "PCM", // WAV files we handle are PCM
		SampleRate:   format.SampleRate,
		Channels:     format.NumChannels,
		BitDepth:     16, // Standard for our use case
		Duration:     dur.Seconds(),
		OriginalSize: size,
		Bitrate:      bitrate,
	}, nil
}

func (s *TranscriptionService) getOpusMetadata(file *os.File, size int64) (AudioMetadata, error) {
	// Standard Opus parameters
	sampleRate := 48000  // Opus default
	channels := 1        // We decode as mono
	frameSize := 960     // Standard Opus frame size for 20ms
	bytesPerFrame := 120 // Typical Opus frame size in bytes

	// Calculate approximate duration
	frameCount := size / int64(bytesPerFrame)
	duration := float64(frameCount) * float64(frameSize) / float64(sampleRate)

	return AudioMetadata{
		Format:       "OPUS",
		Codec:        "Opus",
		SampleRate:   sampleRate,
		Channels:     channels,
		BitDepth:     16, // Opus uses 16-bit samples internally
		Duration:     duration,
		OriginalSize: size,
		Bitrate:      int(float64(size*8) / duration / 1000),
	}, nil
}
