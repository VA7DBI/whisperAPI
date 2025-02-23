// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"fmt"
	"io"
	"os"

	"github.com/amanitaverna/go-mp3"
)

// MP3Format implements the Format interface for MP3 audio files.
type MP3Format struct{}

// GetMetadata extracts metadata from an MP3 file.
func (f *MP3Format) GetMetadata(filename string, fileSize int64) (AudioMetadata, error) {
	file, err := os.Open(filename)
	if err != nil {
		return AudioMetadata{}, err
	}
	defer file.Close()

	decoder, err := mp3.NewDecoder(file)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to create MP3 decoder: %v", err)
	}

	sampleRate := decoder.SampleRate()
	numChannels := 2 // Assuming stereo for MP3

	// Calculate duration
	duration := float64(decoder.Length()) / float64(sampleRate*numChannels*4)

	return AudioMetadata{
		Format:       "MP3",
		Codec:        "MP3",
		SampleRate:   sampleRate,
		Channels:     numChannels,
		BitDepth:     16, // Assuming 16-bit for MP3
		Duration:     duration,
		OriginalSize: fileSize,
		Bitrate:      int(float64(fileSize*8) / duration / 1000),
	}, nil
}

// ConvertToSamples converts an MP3 file to a slice of float32 samples.
func (f *MP3Format) ConvertToSamples(filename string, targetSampleRate int) ([]float32, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder, err := mp3.NewDecoder(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create MP3 decoder: %v", err)
	}

	// Read all samples
	var samples []float32
	buffer := make([]byte, 4096) // Adjust buffer size as needed
	for {
		n, err := decoder.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read MP3 data: %v", err)
		}

		// Convert bytes to float32 samples
		for i := 0; i < n; i += 2 {
			sample := int16(buffer[i]) | int16(buffer[i+1])<<8
			samples = append(samples, float32(sample)/32768.0)
		}
	}

	sampleRate := decoder.SampleRate()
	numChannels := 2 // Assuming stereo for MP3

	// Convert to mono if stereo
	if numChannels > 1 {
		samples = ConvertToMono(samples, numChannels)
	}

	// Resample if needed
	if sampleRate != targetSampleRate {
		samples = ResampleAudio(samples, sampleRate, targetSampleRate)
	}

	return samples, nil
}
