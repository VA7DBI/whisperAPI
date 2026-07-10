// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VA7DBI/whisperAPI/audio"
)

func TestFLACFormatIntegration(t *testing.T) {
	// Test file path
	testFile := filepath.Join("test_fixtures", "test.flac")

	// Check if test file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("test.flac not found, skipping FLAC integration test")
	}

	// Test format detection based on file extension
	ext := strings.ToLower(filepath.Ext(testFile))
	if ext != ".flac" {
		t.Errorf("Expected .flac extension, got %s", ext)
	}

	// Test format instantiation
	var format audio.Format
	switch ext {
	case ".flac":
		format = &audio.FLACFormat{}
	default:
		t.Fatalf("Unexpected extension: %s", ext)
	}

	// Test metadata extraction
	fileInfo, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	metadata, err := format.GetMetadata(testFile, fileInfo.Size())
	if err != nil {
		t.Fatalf("Failed to get FLAC metadata: %v", err)
	}

	// Validate metadata
	if metadata.Format != "FLAC" {
		t.Errorf("Expected format FLAC, got %s", metadata.Format)
	}
	if metadata.Codec != "FLAC" {
		t.Errorf("Expected codec FLAC, got %s", metadata.Codec)
	}
	if metadata.SampleRate == 0 {
		t.Error("Sample rate should not be zero")
	}
	if metadata.Channels == 0 {
		t.Error("Channel count should not be zero")
	}
	if metadata.Duration == 0 {
		t.Error("Duration should not be zero")
	}

	t.Logf("FLAC metadata: Format=%s, Codec=%s, SampleRate=%d, Channels=%d, Duration=%.2fs",
		metadata.Format, metadata.Codec, metadata.SampleRate, metadata.Channels, metadata.Duration)

	// Test sample conversion
	samples, err := format.ConvertToSamples(testFile, 16000)
	if err != nil {
		t.Fatalf("Failed to convert FLAC to samples: %v", err)
	}

	if len(samples) == 0 {
		t.Error("Converted samples should not be empty")
	}

	t.Logf("Successfully converted FLAC to %d samples", len(samples))
}
