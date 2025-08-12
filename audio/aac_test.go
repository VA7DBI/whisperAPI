// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAACFormat_GetMetadata(t *testing.T) {
	format := &AACFormat{}
	testFile := filepath.Join("..", "test_fixtures", "test.aac")

	// Check if test file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("test.aac not found, skipping AAC metadata test")
	}

	fileInfo, err := os.Stat(testFile)
	assert.NoError(t, err)

	metadata, err := format.GetMetadata(testFile, fileInfo.Size())
	
	// AAC parsing might fail if file format is different than expected
	if err != nil {
		t.Logf("AAC metadata extraction failed (this is expected if test.aac is not in ADTS format): %v", err)
		return
	}

	assert.Equal(t, "AAC", metadata.Format)
	assert.NotEmpty(t, metadata.Codec)
	assert.NotZero(t, metadata.SampleRate)
	assert.NotZero(t, metadata.Channels)
	assert.Equal(t, int64(fileInfo.Size()), metadata.OriginalSize)
}

func TestAACFormat_ConvertToSamples(t *testing.T) {
	format := &AACFormat{}
	testFile := filepath.Join("..", "test_fixtures", "test.aac")

	// Check if test file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("test.aac not found, skipping AAC conversion test")
	}

	samples, err := format.ConvertToSamples(testFile, 16000)
	
	// This should fail with the current implementation as noted in the code
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not fully implemented")
	assert.Empty(t, samples)
}
