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
mp3_path = "temp.mp3"
tts.save(mp3_path)

# Convert to audio with the desired format
audio = AudioSegment.from_mp3(mp3_path)
audio = audio.set_frame_rate(16000).set_channels(1).set_sample_width(2)  # 16-bit PCM

# Export as WAV
wav_path = "test.wav"
audio.export(wav_path, format="wav")
print(f"WAV file saved as {wav_path}")

# Export as OGG/Vorbis with explicit format settings
ogg_path = "test.ogg"
audio.export(ogg_path, format="ogg", codec="libvorbis", 
            parameters=["-ar", "16000", "-ac", "1"])
print(f"OGG/Vorbis file saved as {ogg_path}")

# Export as Opus (using ffmpeg directly for better control)
opus_path = "test.opus"
subprocess.run([
    "ffmpeg", "-y",
    "-i", wav_path,
    "-c:a", "libopus",
    "-f", "ogg",  # Explicitly set container format
    "-b:a", "32k",
    "-ar", "48000",  # Opus internal rate
    "-ac", "1",
    "-application", "voip",
    "-frame_duration", "20",
    opus_path
], check=True)
print(f"Opus file saved as {opus_path}")

# Cleanup temporary file
import os
os.remove(mp3_path)