// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiarizeWhisperSegments_DisabledReturnsUnchanged(t *testing.T) {
	segments := []SegmentInfo{{Text: "hello", StartTime: 0, EndTime: 1}}
	samples := make([]float32, 16000)

	result, updated := diarizeWhisperSegments(samples, segments, 16000, DiarizationOptions{Enabled: false})
	assert.Nil(t, result)
	assert.Equal(t, "", updated[0].Speaker)
}

func TestDiarizeWhisperSegments_AssignsSpeakers(t *testing.T) {
	sampleRate := 16000
	samples := make([]float32, sampleRate*4)

	for i := 0; i < sampleRate*2; i++ {
		samples[i] = 0.02
	}
	for i := sampleRate * 2; i < sampleRate*4; i++ {
		samples[i] = 0.3
	}

	segments := []SegmentInfo{
		{Text: "speaker one", StartTime: 0.0, EndTime: 2.0},
		{Text: "speaker two", StartTime: 2.0, EndTime: 4.0},
	}

	result, updated := diarizeWhisperSegments(samples, segments, sampleRate, DiarizationOptions{Enabled: true, ExpectedSpeakers: 2})
	assert.NotNil(t, result)
	assert.Equal(t, true, result.Enabled)
	assert.Equal(t, "acoustic-kmeans", result.Method)
	assert.Equal(t, 2, result.SpeakerCount)
	assert.NotEmpty(t, updated[0].Speaker)
	assert.NotEmpty(t, updated[1].Speaker)
	assert.NotEqual(t, updated[0].Speaker, updated[1].Speaker)
}
