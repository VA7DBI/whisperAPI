// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

const (
	// Time-related constants
	WhisperSampleLength  = 0.02 // Each sample is 20ms in Whisper
	NanosecondsPerSecond = 1_000_000_000
)

// AudioMetadata contains information about an audio file.
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

// Format defines the interface for audio format handlers.
type Format interface {
	GetMetadata(filename string, fileSize int64) (AudioMetadata, error)
	ConvertToSamples(filename string, targetSampleRate int) ([]float32, error)
}

// ConvertToMono converts stereo audio samples to mono.
func ConvertToMono(samples []float32, channels int) []float32 {
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

// ResampleAudio resamples audio samples from one sample rate to another using linear interpolation.
// For production, consider using a better resampling algorithm.
func ResampleAudio(samples []float32, srcRate, dstRate int) []float32 {
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
