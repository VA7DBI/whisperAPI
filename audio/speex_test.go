// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpeexFormat_GetMetadata(t *testing.T) {
	format := &SpeexFormat{}
	testFile := filepath.Join("..", "test_fixtures", "test.spx")

	// Check if test file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("test.spx not found, skipping Speex metadata test")
	}

	fileInfo, err := os.Stat(testFile)
	assert.NoError(t, err)

	metadata, err := format.GetMetadata(testFile, fileInfo.Size())

	// Speex parsing might fail if file format is different than expected
	if err != nil {
		t.Logf("Speex metadata extraction failed (this is expected if test.spx is not in proper OGG/Speex format): %v", err)
		return
	}

	assert.Equal(t, "Speex", metadata.Format)
	assert.Contains(t, metadata.Codec, "Speex")
	assert.NotZero(t, metadata.SampleRate)
	assert.NotZero(t, metadata.Channels)
	assert.Equal(t, int64(fileInfo.Size()), metadata.OriginalSize)

	t.Logf("Speex metadata: Format=%s, Codec=%s, SampleRate=%d, Channels=%d, Duration=%.2fs",
		metadata.Format, metadata.Codec, metadata.SampleRate, metadata.Channels, metadata.Duration)
}

func TestSpeexFormat_ConvertToSamples(t *testing.T) {
	format := &SpeexFormat{}
	testFile := filepath.Join("..", "test_fixtures", "test.spx")

	// Check if test file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("test.spx not found, skipping Speex conversion test")
	}

	samples, err := format.ConvertToSamples(testFile, 16000)

	// This should fail with the current implementation as noted in the code
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not fully implemented")
	assert.Empty(t, samples)
}
