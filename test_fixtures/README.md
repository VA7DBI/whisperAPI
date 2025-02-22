# Test Fixtures

This directory contains test audio files for unit testing.

## Required Files

1. `test.wav` - A WAV file containing speech
   - Format: 16-bit PCM
   - Sample Rate: 16000 Hz
   - Channels: 1 (mono)

2. `test.ogg` - An OGG file containing the same speech
   - Format: OGG container with Vorbis codec
   - Sample Rate: 16000 Hz
   - Channels: 1 (mono)

3. `test.opus` - An Opus-encoded file
   - Format: OGG container with Opus codec
   - Sample Rate: 48000 Hz (internal)
   - Output Rate: 16000 Hz
   - Channels: 1 (mono)
   - Frame Size: 20ms
   - Bitrate: 32 kbps

## Generating Test Files

Install required dependencies:
```bash
# Install Python dependencies
pip install gtts pydub

# Install ffmpeg (required for Opus encoding)
# On Debian/Ubuntu:
sudo apt-get install ffmpeg
# On FreeBSD:
pkg install ffmpeg
```

Generate all test files:
```bash
python makewave.py "This is a test recording"
```

The script will create:
- test.wav (PCM WAV)
- test.ogg (OGG/Vorbis)
- test.opus (OGG/Opus)
