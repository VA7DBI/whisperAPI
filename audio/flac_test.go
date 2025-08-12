// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFLACFormat_GetMetadata(t *testing.T) {
	testFile := filepath.Join("..", "test_fixtures", "test.flac")

	fileInfo, err := os.Stat(testFile)
	if err != nil {
		t.Skipf("Test FLAC file not found at %s - please add test fixture", testFile)
		return
	}
	fileSize := fileInfo.Size()

	format := &FLACFormat{}
	metadata, err := format.GetMetadata(testFile, fileSize)
	assert.NoError(t, err)

	assert.Equal(t, "FLAC", metadata.Format)
	assert.Equal(t, "FLAC", metadata.Codec)
	assert.NotEmpty(t, metadata.SampleRate)
	assert.NotEmpty(t, metadata.Channels)
	assert.NotEmpty(t, metadata.BitDepth)
	assert.NotEmpty(t, metadata.Duration)
	assert.Equal(t, fileSize, metadata.OriginalSize)
	assert.NotEmpty(t, metadata.Bitrate)
}

func TestFLACFormat_ConvertToSamples(t *testing.T) {
	testFile := filepath.Join("..", "test_fixtures", "test.flac")

	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("Test FLAC file not found at %s - please add test fixture", testFile)
		return
	}

	targetSampleRate := 16000

	format := &FLACFormat{}
	samples, err := format.ConvertToSamples(testFile, targetSampleRate)
	assert.NoError(t, err)
	assert.NotEmpty(t, samples)

	// Verify samples are in reasonable range (normalized to [-1, 1])
	for _, sample := range samples[:100] { // Check first 100 samples
		assert.True(t, sample >= -1.0 && sample <= 1.0, "Sample out of range: %f", sample)
	}
}
