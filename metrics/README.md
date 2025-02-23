# Metrics Package

Package metrics provides Prometheus metrics collection for the Whisper API service.

## Available Metrics

### Counters
- `whisperapi_transcription_requests_total{status="success|error",format="wav|mp3|ogg|opus"}`
  - Total number of transcription requests by status and format

### Histograms
- `whisperapi_transcription_duration_seconds`
  - Time taken to process transcription requests
- `whisperapi_audio_duration_seconds`
  - Duration of processed audio files
- `whisperapi_cpu_time_seconds{operation="user|system|total"}`
  - CPU time spent processing requests

### Gauges
- `whisperapi_memory_usage_bytes{type="allocated|system|heap"}`
  - Memory usage statistics

## Usage Example

