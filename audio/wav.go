// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"fmt"
	"os"

	"github.com/go-audio/wav"
)

type WAVFormat struct{}

func (f *WAVFormat) GetMetadata(filename string, fileSize int64) (AudioMetadata, error) {
	file, err := os.Open(filename)
	if err != nil {
		return AudioMetadata{}, err
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return AudioMetadata{}, fmt.Errorf("invalid WAV file")
	}

	format := decoder.Format()
	dur, err := decoder.Duration()
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to get duration: %v", err)
	}

	duration := dur.Seconds()
	// Calculate bitrate from file size and duration
	bitrate := int(float64(fileSize*8) / duration / 1000)

	return AudioMetadata{
		Format:       "WAV",
		Codec:        "PCM",
		SampleRate:   format.SampleRate,
		Channels:     format.NumChannels,
		BitDepth:     16, // WAV files are typically 16-bit PCM
		Duration:     duration,
		OriginalSize: fileSize,
		Bitrate:      bitrate,
	}, nil
}

func (f *WAVFormat) ConvertToSamples(filename string, targetSampleRate int) ([]float32, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}

	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to read PCM buffer: %v", err)
	}

	// Convert int buffer to float32 samples
	numSamples := len(buf.Data)
	samples := make([]float32, numSamples)

	// Scale factor for audio format
	scale := float32(1.0 / 32768.0)

	for i, sample := range buf.Data {
		samples[i] = float32(sample) * scale
	}

	format := decoder.Format()
	if format.NumChannels > 1 {
		samples = ConvertToMono(samples, format.NumChannels)
	}

	// Resample if needed
	if format.SampleRate != targetSampleRate {
		samples = ResampleAudio(samples, format.SampleRate, targetSampleRate)
	}

	return samples, nil
}
