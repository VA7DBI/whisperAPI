// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMP3Format_GetMetadata(t *testing.T) {
	testFile := filepath.Join("..", "test_fixtures", "test.mp3") // Corrected path

	fileInfo, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}
	fileSize := fileInfo.Size()

	format := &MP3Format{}
	metadata, err := format.GetMetadata(testFile, fileSize)
	assert.NoError(t, err)

	assert.Equal(t, "MP3", metadata.Format)
	assert.Equal(t, "MP3", metadata.Codec)
	assert.NotEmpty(t, metadata.SampleRate)
	assert.NotEmpty(t, metadata.Channels)
	assert.Equal(t, 16, metadata.BitDepth)
	assert.NotEmpty(t, metadata.Duration)
	assert.Equal(t, fileSize, metadata.OriginalSize)
	assert.NotEmpty(t, metadata.Bitrate)
}

func TestMP3Format_ConvertToSamples(t *testing.T) {
	testFile := filepath.Join("..", "test_fixtures", "test.mp3") // Corrected path
	targetSampleRate := 16000

	format := &MP3Format{}
	samples, err := format.ConvertToSamples(testFile, targetSampleRate)
	assert.NoError(t, err)
	assert.NotEmpty(t, samples)
}
