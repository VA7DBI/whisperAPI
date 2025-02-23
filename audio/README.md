# Audio Package

Package audio provides format-specific handlers for audio processing in the Whisper API service.

## Supported Formats

- WAV (PCM): 16-bit linear PCM audio
- MP3: MPEG Layer-3 audio
- OGG Vorbis: Vorbis codec in OGG container
- Opus (SILK): Speech-optimized Opus using SILK codec

## Format Handlers

Each audio format implements the `Format` interface:

