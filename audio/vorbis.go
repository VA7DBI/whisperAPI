// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"fmt"
	"io"
	"os"

	"github.com/jfreymuth/oggvorbis"
)

// VorbisFormat implements the Format interface for OGG Vorbis audio files.
type VorbisFormat struct{}

// GetMetadata extracts metadata from an OGG Vorbis file.
func (f *VorbisFormat) GetMetadata(filename string, fileSize int64) (AudioMetadata, error) {
	file, err := os.Open(filename)
	if err != nil {
		return AudioMetadata{}, err
	}
	defer file.Close()

	decoder, err := oggvorbis.NewReader(file)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to create Vorbis decoder: %v", err)
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
		OriginalSize: fileSize,
		Bitrate:      int(float64(fileSize*8) / duration / 1000),
	}, nil
}

// ConvertToSamples converts an OGG Vorbis file to a slice of float32 samples.
func (f *VorbisFormat) ConvertToSamples(filename string, targetSampleRate int) ([]float32, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder, err := oggvorbis.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vorbis decoder: %v", err)
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
		samples = ConvertToMono(samples, decoder.Channels())
	}

	// Resample if needed
	if decoder.SampleRate() != targetSampleRate {
		samples = ResampleAudio(samples, decoder.SampleRate(), targetSampleRate)
	}

	return samples, nil
}
