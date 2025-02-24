// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/pion/opus"
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

	// Create Opus decoder
	decoder := opus.NewDecoder()
	if err != nil {
		return nil, fmt.Errorf("failed to create Opus decoder: %v", err)
	}

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
			outputPCM := make([]byte, frameSize*2)

			// Decode Opus frame
			_, _, err := decoder.Decode(packet, outputPCM)
			if err != nil {
				fmt.Printf("Warning: failed to decode frame %d: %v\n", i, err)
				continue
			}

			// Convert decoded bytes to float32 samples
			frame := make([]float32, frameSize)
			for i := 0; i < frameSize; i++ {
				sample := int16(outputPCM[i*2]) | (int16(outputPCM[i*2+1]) << 8)
				frame[i] = float32(sample) / 32768.0
			}

			pcm = append(pcm, frame...)
		}
	}

	if len(pcm) == 0 {
		return nil, fmt.Errorf("no valid Opus frames decoded")
	}

	// Resample from 48kHz to target rate
	return ResampleAudio(pcm, 48000, targetSampleRate), nil
}
