//go:build cgo

// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"fmt"
	"os"

	oggopus "github.com/altager/oggopus"
	"layeh.com/gopus"
)

// OpusFormat implements the Format interface for Opus audio files.
type OpusFormat struct{}

// GetMetadata extracts metadata from an Opus file.
func (f *OpusFormat) GetMetadata(filename string, fileSize int64) (AudioMetadata, error) {
	// Standard Opus parameters
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
func (f *OpusFormat) ConvertToSamples(filename string, targetSampleRate int) ([]float32, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dec, err := oggopus.NewOpusReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create oggopus OpusReader: %v", err)
	}

	var pcm []float32
	frameSize := 960 // 20ms at 48kHz
	var decoder *gopus.Decoder

	for {
		packet, err := dec.NextPacket()
		if err != nil {
			break
		}

		// Initialize decoder after headers are read (on first packet)
		if decoder == nil {
			decoder, err = gopus.NewDecoder(int(dec.InputSampleRate), int(dec.ChannelCount))
			if err != nil {
				return nil, fmt.Errorf("failed to create Opus decoder: %v", err)
			}
		}

		// Decode Opus packet to PCM samples
		output, err := decoder.Decode(packet.PacketData, frameSize, false)
		if err != nil {
			continue // Skip invalid packets
		}

		// Convert int16 to float32 and handle mono conversion
		for i := 0; i < len(output); i += int(dec.ChannelCount) {
			sample := float32(output[i]) / 32768.0
			pcm = append(pcm, sample)
		}
	}

	if len(pcm) == 0 {
		return nil, fmt.Errorf("no valid Opus frames decoded")
	}

	// Resample from decoded sample rate to target rate
	return ResampleAudio(pcm, int(dec.InputSampleRate), targetSampleRate), nil
}
