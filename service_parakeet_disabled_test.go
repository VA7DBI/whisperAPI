// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranscribeWithEngine_ParakeetDisabled(t *testing.T) {
	s := &TranscriptionService{
		parakeetDisabled:      true,
		parakeetDisableReason: "no parakeet binary found",
	}

	text, segments, confidence, diarization, err := s.transcribeWithEngine(EngineParakeet, "audio.wav", nil, DiarizationOptions{})
	assert.Error(t, err)
	assert.Empty(t, text)
	assert.Empty(t, segments)
	assert.Equal(t, float64(0), confidence)
	assert.Nil(t, diarization)
	assert.Contains(t, err.Error(), "parakeet engine is disabled")
}
