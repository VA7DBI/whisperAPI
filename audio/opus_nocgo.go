//go:build !cgo

// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"fmt"
)

// OpusFormat implements the Format interface for Opus audio files.
// This is a stub implementation when CGO is not available.
type OpusFormat struct{}

// GetMetadata extracts metadata from an Opus file.
func (f *OpusFormat) GetMetadata(filename string, fileSize int64) (AudioMetadata, error) {
	// Standard Opus parameters (estimated)
	sampleRate := 48000  // Opus default
	channels := 1        // We decode as mono
	frameSize := 960     // Standard Opus frame size for 20ms
	bytesPerFrame := 120 // Typical Opus frame size in bytes

	// Calculate approximate duration
	frameCount := fileSize / int64(bytesPerFrame)
	duration := float64(frameCount) * float64(frameSize) / float64(sampleRate)

	return AudioMetadata{
		Format:       "OPUS",
		Codec:        "Opus",
		SampleRate:   sampleRate,
		Channels:     channels,
		BitDepth:     16, // Opus uses 16-bit samples internally
		Duration:     duration,
		OriginalSize: fileSize,
		Bitrate:      int(float64(fileSize*8) / duration / 1000),
	}, nil
}

// ConvertToSamples converts an Opus file to a slice of float32 samples.
// This is a stub implementation when CGO is not available.
func (f *OpusFormat) ConvertToSamples(filename string, targetSampleRate int) ([]float32, error) {
	return nil, fmt.Errorf("Opus decoding requires CGO to be enabled and a C compiler to be available")
}
