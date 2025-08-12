# Audio Package

Package audio provides format-specific handlers for audio processing in the Whisper API service.

## Supported Formats

- WAV (PCM): 16-bit linear PCM audio
- MP3: MPEG Layer-3 audio
- FLAC: Free Lossless Audio Codec for high-quality audio
- AAC: Advanced Audio Coding (metadata parsing only - decoding requires external tools)
- OGG Vorbis: Vorbis codec in OGG container
- Opus (SILK): Speech-optimized Opus using SILK codec

## Format Handlers

Each audio format implements the `Format` interface:

