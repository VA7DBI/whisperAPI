# Audio Package

Package audio provides format-specific handlers for audio processing in the Whisper API service.

## Supported Formats

- WAV (PCM): 16-bit linear PCM audio
- MP3: MPEG Layer-3 audio
- FLAC: Free Lossless Audio Codec for high-quality audio
- AAC: Advanced Audio Coding (metadata parsing only - decoding requires external tools)
- Speex: Speech codec optimized for voice (metadata parsing only - full support available with CGO)
- OGG Vorbis: Vorbis codec in OGG container
- Opus (SILK): Speech-optimized Opus using SILK codec

### Full Speex Support

For complete Speex audio decoding, you can use `github.com/chinatcp/go-speex`:

**Requirements:**
- CGO enabled (`CGO_ENABLED=1`)
- C compiler (gcc)
- libspeex development libraries

**Installation steps:**
1. Install libspeex: `sudo apt-get install libspeex-dev` (Ubuntu/Debian)
2. Enable CGO: `export CGO_ENABLED=1`
3. Add dependency: `go get github.com/chinatcp/go-speex`

**Note:** This adds complexity to builds and deployment. Consider using external tools like ffmpeg for simpler deployment.

## Format Handlers

Each audio format implements the `Format` interface:

