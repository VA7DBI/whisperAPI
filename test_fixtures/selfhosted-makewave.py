## Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
## SPDX-License-Identifier: BSD-3-Clause
 

import sys
from TTS.api import TTS
from pydub import AudioSegment

# Check if text is provided
if len(sys.argv) < 2:
    print("Usage: python generate_wav.py \"Your text here\"")
    sys.exit(1)

# Get text from command-line arguments
text = " ".join(sys.argv[1:])

#Select a male American voice from Coqui TTS
# model_name = "tts_models/en/ljspeech/tacotron2-DDC"

#need to experment with this voice more
# model_name = "tts_models/en/vctk/vits"

model_name = "tts_models/en/ljspeech/glow-tts"

tts = TTS(model_name)

# Generate and save speech
wav_path = "test.wav"
tts.tts_to_file(text=text, file_path=wav_path)

# Convert to desired format (16-bit PCM, 16kHz, mono)
audio = AudioSegment.from_wav(wav_path)
audio = audio.set_frame_rate(16000).set_channels(1).set_sample_width(2)
audio.export(wav_path, format="wav")

print(f"WAV file saved as {wav_path}")
