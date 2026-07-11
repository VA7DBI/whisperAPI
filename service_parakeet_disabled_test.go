// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsParakeetBinaryNotFoundError(t *testing.T) {
	assert.False(t, isParakeetBinaryNotFoundError(assert.AnError))
	assert.True(t, isParakeetBinaryNotFoundError(
		errorString("no parakeet binary found. install it in PATH"),
	))
	assert.True(t, isParakeetBinaryNotFoundError(
		errorString("parakeet binary not found at configured path \"/tmp/parakeet\""),
	))
	assert.False(t, isParakeetBinaryNotFoundError(
		errorString("failed to start embedded parakeet process: permission denied"),
	))
}

func TestTranscribeWithEngine_ParakeetDisabled(t *testing.T) {
	s := &TranscriptionService{
		parakeetDisabled:      true,
		parakeetDisableReason: "no parakeet binary found",
	}

	text, segments, confidence, err := s.transcribeWithEngine(EngineParakeet, "audio.wav", nil)
	assert.Error(t, err)
	assert.Empty(t, text)
	assert.Empty(t, segments)
	assert.Equal(t, float64(0), confidence)
	assert.Contains(t, err.Error(), "parakeet engine is disabled")
}

type errorString string

func (e errorString) Error() string {
	return string(e)
}
