# Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
# SPDX-License-Identifier: BSD-3-Clause
 
import sys
from gtts import gTTS
from pydub import AudioSegment
import subprocess

# Check if text is provided
if len(sys.argv) < 2:
    print("Usage: python makewave.py \"Your text here\"")
    sys.exit(1)

# Get text from command-line arguments
text = " ".join(sys.argv[1:])

# Generate speech using gTTS
tts = gTTS(text, lang="en")

# Save as an MP3 file temporarily
mp3_path = "test.mp3"
tts.save(mp3_path)

# Convert to audio with the desired format
audio = AudioSegment.from_mp3(mp3_path)
audio = audio.set_frame_rate(48000).set_channels(1).set_sample_width(2)  # 16-bit PCM

# Export as WAV
wav_path = "test.wav"
audio.export(wav_path, format="wav")
print(f"WAV file saved as {wav_path}")

# Export as OGG/Vorbis with explicit format settings
ogg_path = "test.ogg"
audio.export(ogg_path, format="ogg", codec="libvorbis", 
            parameters=["-ar", "16000", "-ac", "1"])
print(f"OGG/Vorbis file saved as {ogg_path}")

# Export as Opus (using opusenc directly)
opus_path = "test.opus"
subprocess.run([
    "opusenc",
    "--bitrate", "64",
    wav_path,
    opus_path
], check=True)
print(f"Opus file saved as {opus_path}")

# Export as FLAC
flac_path = "test.flac"
audio.export(flac_path, format="flac", parameters=["-ar", "16000", "-ac", "1"])
print(f"FLAC file saved as {flac_path}")

# Export as AAC (requires ffmpeg)
aac_path = "test.aac"
try:
    audio.export(aac_path, format="aac", codec="aac", 
                parameters=["-ar", "16000", "-ac", "1", "-b:a", "128k"])
    print(f"AAC file saved as {aac_path}")
except Exception as e:
    print(f"Warning: Failed to create AAC file (ffmpeg/aac encoder may not be available): {e}")

print("\nAll audio files generated successfully!")