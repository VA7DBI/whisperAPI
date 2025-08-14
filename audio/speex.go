// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// SpeexFormat implements the Format interface for Speex audio files.
type SpeexFormat struct{}

// SpeexHeader represents the Speex header structure
type SpeexHeader struct {
	SpeexString      [8]byte  // "Speex   "
	SpeexVersion     [20]byte // Version string
	SpeexVersionID   uint32   // Version ID
	HeaderSize       uint32   // Header size
	Rate             uint32   // Sample rate
	Mode             uint32   // Encoding mode (0=narrowband, 1=wideband, 2=ultra-wideband)
	ModeBitstreamVersion uint32 // Mode bitstream version
	Channels         uint32   // Number of channels
	Bitrate          int32    // Bitrate (-1 if VBR)
	FrameSize        uint32   // Frame size
	VBR              uint32   // VBR flag
	FramesPerPacket  uint32   // Frames per Ogg packet
	ExtraHeaders     uint32   // Extra headers
	Reserved1        uint32   // Reserved
	Reserved2        uint32   // Reserved
}

// GetMetadata extracts metadata from a Speex file.
func (f *SpeexFormat) GetMetadata(filename string, fileSize int64) (AudioMetadata, error) {
	file, err := os.Open(filename)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to open Speex file: %v", err)
	}
	defer file.Close()

	// Read and verify OGG header first
	oggHeader := make([]byte, 4)
	_, err = file.Read(oggHeader)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to read OGG header: %v", err)
	}

	if !bytes.Equal(oggHeader, []byte("OggS")) {
		return AudioMetadata{}, fmt.Errorf("not a valid OGG file")
	}

	// Skip to the Speex header (this is a simplified approach)
	// In a real implementation, we would properly parse the OGG container
	_, err = file.Seek(28, io.SeekStart) // Skip OGG page header
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to seek to Speex header: %v", err)
	}

	// Read Speex header
	var header SpeexHeader
	err = binary.Read(file, binary.LittleEndian, &header)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("failed to read Speex header: %v", err)
	}

	// Verify Speex magic string
	speexMagic := string(header.SpeexString[:])
	if speexMagic != "Speex   " {
		return AudioMetadata{}, fmt.Errorf("not a valid Speex file: invalid magic string")
	}

	// Calculate approximate duration
	var duration float64
	if header.Rate > 0 && header.Bitrate > 0 {
		// Estimate duration from file size and bitrate
		duration = float64(fileSize*8) / float64(header.Bitrate)
	}

	// Determine mode description
	var modeDesc string
	switch header.Mode {
	case 0:
		modeDesc = "Narrowband (8kHz)"
	case 1:
		modeDesc = "Wideband (16kHz)"
	case 2:
		modeDesc = "Ultra-wideband (32kHz)"
	default:
		modeDesc = fmt.Sprintf("Unknown mode %d", header.Mode)
	}

	return AudioMetadata{
		Duration:     duration,
		SampleRate:   int(header.Rate),
		Channels:     int(header.Channels),
		Bitrate:      int(header.Bitrate),
		Format:       "Speex",
		Codec:        fmt.Sprintf("Speex %s", modeDesc),
		BitDepth:     16, // Speex internally uses floating point, equivalent to ~16-bit
		OriginalSize: fileSize,
	}, nil
}

// ConvertToSamples converts Speex audio data to float32 samples at the target sample rate.
func (f *SpeexFormat) ConvertToSamples(filename string, targetSampleRate int) ([]float32, error) {
	// Note: Speex decoding requires libspeex bindings or external tools.
	// The Go ecosystem has limited native Speex decoders.
	// For full Speex support, consider:
	// 1. CGO bindings like github.com/chinatcp/go-speex (requires gcc and libspeex)
	// 2. External tools like ffmpeg
	// 3. speexdec command-line tool
	// 
	// To enable github.com/chinatcp/go-speex:
	// - Install gcc compiler
	// - Install libspeex development libraries
	// - Enable CGO_ENABLED=1
	// - Import "github.com/chinatcp/go-speex/speex"
	
	return nil, fmt.Errorf("Speex audio decoding is not fully implemented yet - requires CGO bindings (github.com/chinatcp/go-speex) or external tools like ffmpeg. Consider using speexdec for conversion to WAV first")
}
