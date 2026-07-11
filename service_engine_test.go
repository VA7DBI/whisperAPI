// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTranscriptionEngine_DefaultsToWhisper(t *testing.T) {
	engine, err := parseTranscriptionEngine("")
	assert.NoError(t, err)
	assert.Equal(t, EngineWhisper, engine)
}

func TestParseTranscriptionEngine_NormalizesInput(t *testing.T) {
	engine, err := parseTranscriptionEngine("  WHISPER ")
	assert.NoError(t, err)
	assert.Equal(t, EngineWhisper, engine)
}

func TestParseTranscriptionEngine_Parakeet(t *testing.T) {
	engine, err := parseTranscriptionEngine("PARAKEET")
	assert.NoError(t, err)
	assert.Equal(t, EngineParakeet, engine)
}

func TestParseTranscriptionEngine_Unsupported(t *testing.T) {
	_, err := parseTranscriptionEngine("faster-whisper")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported engine")
	assert.Contains(t, err.Error(), "parakeet")
}
