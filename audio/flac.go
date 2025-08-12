// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"fmt"
	"io"
	"os"

	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/frame"
	"github.com/mewkiz/flac/meta"
)

// FLACFormat implements the Format interface for FLAC audio files.
type FLACFormat struct{}

// GetMetadata extracts metadata from a FLAC file.
func (f *FLACFormat) GetMetadata(filename string, fileSize int64) (AudioMetadata, error) {
	stream, err := flac.ParseFile(filename)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to parse FLAC file: %v", err)
	}
	defer stream.Close()

	// Get StreamInfo from the stream
	streamInfo := stream.Info
	if streamInfo == nil {
		return AudioMetadata{}, fmt.Errorf("no StreamInfo found in FLAC stream")
	}

	// Calculate duration
	var durationSeconds float64
	if streamInfo.SampleRate > 0 {
		durationSeconds = float64(streamInfo.NSamples) / float64(streamInfo.SampleRate)
	}

	// Calculate bitrate (approximate)
	var bitrate int
	if durationSeconds > 0 {
		bitrate = int((fileSize * 8) / int64(durationSeconds) / 1000)
	}

	return AudioMetadata{
		Duration:   durationSeconds,
		SampleRate: int(streamInfo.SampleRate),
		Channels:   int(streamInfo.NChannels),
		Bitrate:    bitrate,
		Format:     "FLAC",
		Codec:      "FLAC",
		BitDepth:   int(streamInfo.BitsPerSample),
		OriginalSize: fileSize,
	}, nil
}

// ConvertToSamples converts FLAC audio data to float32 samples at the target sample rate.
func (f *FLACFormat) ConvertToSamples(filename string, targetSampleRate int) ([]float32, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open FLAC file: %v", err)
	}
	defer file.Close()

	stream, err := flac.New(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create FLAC stream: %v", err)
	}

	// Get StreamInfo from the stream
	streamInfo := stream.Info
	if streamInfo == nil {
		return nil, fmt.Errorf("no StreamInfo found in FLAC stream")
	}

	var allSamples []float32

	// Read frames and decode audio samples
	for {
		frame, err := stream.ParseNext()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to parse frame: %v", err)
		}

		frameSamples, err := f.convertFLACFrame(frame, streamInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to convert frame: %v", err)
		}

		allSamples = append(allSamples, frameSamples...)
	}

	// Resample to target rate if necessary
	if streamInfo.SampleRate != uint32(targetSampleRate) {
		allSamples = ResampleAudio(allSamples, int(streamInfo.SampleRate), targetSampleRate)
	}

	// Convert to mono if necessary
	if streamInfo.NChannels > 1 {
		allSamples = ConvertToMono(allSamples, int(streamInfo.NChannels))
	}

	return allSamples, nil
}

// convertFLACFrame converts a FLAC frame to float32 samples.
func (f *FLACFormat) convertFLACFrame(frame *frame.Frame, streamInfo *meta.StreamInfo) ([]float32, error) {
	if len(frame.Subframes) == 0 {
		return nil, fmt.Errorf("frame has no subframes")
	}

	blockSize := int(frame.Header.BlockSize)
	nChannels := int(streamInfo.NChannels)
	bitsPerSample := int(streamInfo.BitsPerSample)

	// Create sample buffer
	samples := make([][]int32, nChannels)
	for i := range samples {
		samples[i] = make([]int32, blockSize)
	}

	// Decode samples from subframes
	for ch, subframe := range frame.Subframes {
		if ch >= nChannels {
			break
		}
		for i := 0; i < blockSize && i < len(subframe.Samples); i++ {
			samples[ch][i] = subframe.Samples[i]
		}
	}

	// Convert to float32 samples
	result := make([]float32, blockSize)
	
	// Scale factor to convert from integer samples to float32 range [-1.0, 1.0]
	maxValue := int32(1 << (bitsPerSample - 1))
	scale := float32(1.0) / float32(maxValue)
	
	if nChannels == 1 {
		// Mono audio
		for i := 0; i < blockSize; i++ {
			sample := samples[0][i]
			result[i] = float32(sample) * scale
		}
	} else {
		// Multi-channel audio - mix to mono
		for i := 0; i < blockSize; i++ {
			var sum int64
			for ch := 0; ch < nChannels; ch++ {
				sum += int64(samples[ch][i])
			}
			// Average the channels
			avg := sum / int64(nChannels)
			result[i] = float32(avg) * scale
		}
	}

	return result, nil
}
