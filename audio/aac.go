// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"fmt"
	"io"
	"os"

	"github.com/Comcast/gaad"
)

// AACFormat implements the Format interface for AAC audio files.
type AACFormat struct{}

// GetMetadata extracts metadata from an AAC file.
func (f *AACFormat) GetMetadata(filename string, fileSize int64) (AudioMetadata, error) {
	file, err := os.Open(filename)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to open AAC file: %v", err)
	}
	defer file.Close()

	// Read the file to parse ADTS header
	data, err := io.ReadAll(file)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to read AAC file: %v", err)
	}

	// Parse ADTS header
	adts, err := gaad.ParseADTS(data)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to parse ADTS: %v", err)
	}

	// Extract metadata from ADTS
	channels := int(adts.ChannelConfiguration)
	if channels == 0 {
		// Default to mono if channel configuration is not defined
		channels = 1
	}

	// Calculate approximate duration
	var duration float64
	if adts.Bitrate > 0 && adts.SamplingFrequency > 0 {
		// Estimate duration from file size and bitrate
		duration = float64(fileSize*8) / float64(adts.Bitrate)
	}

	// Get profile description
	var codec string
	if int(adts.Profile) < len(gaad.AACProfileType) {
		codec = gaad.AACProfileType[adts.Profile]
	} else {
		codec = "AAC"
	}

	return AudioMetadata{
		Duration:     duration,
		SampleRate:   int(adts.SamplingFrequency),
		Channels:     channels,
		Bitrate:      int(adts.Bitrate),
		Format:       "AAC",
		Codec:        codec,
		BitDepth:     16, // AAC typically uses 16-bit equivalent
		OriginalSize: fileSize,
	}, nil
}

// ConvertToSamples converts AAC audio data to float32 samples at the target sample rate.
func (f *AACFormat) ConvertToSamples(filename string, targetSampleRate int) ([]float32, error) {
	// Note: The gaad library appears to be primarily a parser for ADTS headers
	// and doesn't provide full AAC decoding capabilities. For full AAC decoding,
	// we would need a different library or external tool like ffmpeg.
	// For now, we'll return an error indicating that AAC decoding is not fully implemented.
	
	return nil, fmt.Errorf("AAC audio decoding is not fully implemented yet - the gaad library only provides ADTS parsing. Consider using ffmpeg or another decoder library for full AAC support")
}
