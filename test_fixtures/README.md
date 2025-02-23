# Test Fixtures

This directory contains test audio files and scripts for testing the Whisper API.

## File Types

- `test.wav`: 16-bit PCM WAV test file
- `test.mp3`: MP3-encoded test file
- `test.ogg`: Vorbis-encoded test file
- `test.opus`: SILK-encoded Opus test file

## Test Audio Generation

Use `makewave.py` to generate test files in all supported formats:
```bash
# Install Python dependencies
pip install gtts pydub

# Install opus-tools (required for Opus encoding)
# On Debian/Ubuntu:
sudo apt-get install opus-tools
# On FreeBSD:
pkg install opus-tools
```

Generate all test files:
```bash
python makewave.py "This is a test recording"
```

The script will create:
- test.wav (PCM WAV)
- test.ogg (OGG/Vorbis)
- test.opus (OGG/Opus)
- test.mp3