# Whisper API Service

A self-hosted voice-to-text transcription API service using Whisper AI. Supports multiple audio formats including WAV, MP3, OGG (Vorbis), and Opus.

## Features

- Speech-to-text transcription using Whisper AI
- Support for multiple audio formats:
  - WAV (16-bit PCM)
  - MP3 (MPEG Layer-3)
  - FLAC (Free Lossless Audio Codec)
  - AAC (Advanced Audio Coding) - metadata parsing only
  - Speex (Speech codec) - metadata parsing only
  - OGG/Vorbis
  - OGG/Opus
- Automatic format detection and conversion:
  - Sample rate conversion to 16kHz
  - Mono channel conversion
  - Bit depth normalization
- Rich metadata for each transcription:
  - Word-level timing
  - Confidence scores
  - Audio format details
  - Performance metrics
- Prometheus monitoring with detailed metrics
- Swagger API documentation
- Authentication:
  - Bearer token authentication
  - Multi-layer token validation:
    - Redis cache (fast)
    - PostgreSQL database (persistent)
    - Static tokens (fallback)
  - Configurable token expiration
  - Optional authentication mode

## Prerequisites

Core requirements:

- Go 1.20 or later
- Whisper model file (`ggml-base.bin`) from [ggerganov/whisper.cpp on Hugging Face](https://huggingface.co/ggerganov/whisper.cpp)

For test fixtures generation:

- Python 3.x
- FFmpeg
- gTTS (`pip install gtts`)
- pydub (`pip install pydub`)

Additional requirements for authentication:

- Redis (optional, for token caching)
- PostgreSQL (optional, for token storage)

## Quick Start

1. Clone the repository:

   ```bash
   git clone https://github.com/VA7DBI/whisperAPI.git
   cd whisperAPI
   ```

1. Download the Whisper model:

   ```bash
   mkdir models
   curl -L https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin -o models/ggml-base.bin
   ```

1. Optional: download local Parakeet backend models (for `engine=parakeet` deployments):

  ```bash
  make models        # int8 Parakeet models (default)
  # or
  make models-fp32   # fp32 Parakeet models
  ```

1. Install dependencies:

   ```bash
   go mod download
   ```

1. Generate test fixtures (optional):

   ```bash
   cd test_fixtures
   python makewave.py "This is a test audio file"
   cd ..
   ```

1. Run API tests (optional):

  ```bash
  go test -v ./...
  ```

Note, if whisper.h is not in your default system include path, you may need to use the `CGO_*FLAGS` environment variables before running your `go test` or `go build`, for example:

```bash
export CGO_CFLAGS="-I/usr/local/include"
export CGO_LDFLAGS="-L/usr/local/lib"
```

1. Build and run:

   ```bash
   go build
   ./whisperAPI
   ```

### Authentication Setup

1. Create the PostgreSQL token table:

```sql
CREATE TABLE api_tokens (
    token VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    valid_until TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    scopes TEXT[]
);

-- Example token insertion
INSERT INTO api_tokens (token, user_id, valid_until, scopes) 
VALUES (
    'your-api-token-here',
    'user123',
    NOW() + INTERVAL '30 days',
    ARRAY['transcribe', 'read']
);
```

1. Configure authentication in `config.yaml`:

```yaml
auth:
  enabled: true  # Enable/disable auth
  tokens:        # Static fallback tokens
    - "your-static-token"
  redis:
    enabled: true
    host: "localhost"
    port: 6379
    db: 0
    password: ""
    key_ttl: 3600  # Cache TTL in seconds
  postgres:
    enabled: true
    host: "localhost"
    port: 5432
    user: "postgres"
    password: "secret"
    dbname: "whisperapi"
    table: "api_tokens"
    query: "SELECT EXISTS(SELECT 1 FROM api_tokens WHERE token = $1 AND valid_until > NOW())"
```

### CORS Setup

The API supports configurable CORS via the `cors` section in `config.yaml`.

Development (allow-all origins):

```yaml
cors:
  enabled: true
  allow_all_origins: true
  allowed_origins: []
  allowed_methods:
    - GET
    - POST
    - PUT
    - PATCH
    - DELETE
    - HEAD
    - OPTIONS
  allowed_headers:
    - Origin
    - Content-Type
    - Content-Length
    - Authorization
    - X-API-Key
  expose_headers:
    - Content-Length
```

Production (restrict to known frontend origins):

```yaml
cors:
  enabled: true
  allow_all_origins: false
  allowed_origins:
    - "https://app.example.com"
    - "https://admin.example.com"
  allowed_methods:
    - GET
    - POST
    - OPTIONS
  allowed_headers:
    - Content-Type
    - Authorization
    - X-API-Key
  expose_headers:
    - Content-Length
```

Notes:

- Browser preflight (`OPTIONS`) requests are handled automatically when CORS is enabled.
- If `allow_all_origins` is `false`, requests are only accepted from `allowed_origins`.
- Keep `allow_all_origins: true` for development only.

Model download notes:

- `make models` downloads Parakeet int8 model artifacts to `./models`.
- `make models-silero-vad` downloads `silero_vad.onnx` and verifies it with a pinned sha256 checksum.
- These targets do not install ONNX Runtime system libraries.

## API Documentation

### GET /swagger/

Swagger UI for API documentation.

### GET /health

Health check endpoint.

Response:

```json
{
  "status": "ok",
  "parakeet_enabled": true,
  "parakeet_mode": "embedded"
}
```

### GET /metrics

Prometheus metrics endpoint providing:

- Request counts by format and status
- Processing durations (histogram)
- Audio durations (histogram)
- Memory usage (gauge)
- CPU/GPU time (histogram)

### POST /transcribe

Upload an audio file for transcription.

Request:

- Method: POST
- Content-Type: multipart/form-data
- Form field: "audio" (file)
- Optional form field: "engine" (string, defaults to "whisper")
- Supported formats: WAV, OGG/Vorbis, OGG/Opus
- Supported engines: whisper, parakeet

Parakeet engine configuration (optional):

```yaml
parakeet:
  endpoint: ""  # Optional external endpoint. Leave empty to use in-process embedded runtime.
  timeout_seconds: 30
  language: en
  model: parakeet-tdt-0.6b
  embedded:
    enabled: true
    models_dir: models
    workers: 2
```

Embedded mode behavior:

- If `parakeet.endpoint` is empty and `parakeet.embedded.enabled` is true, whisperAPI initializes an in-process ONNX Parakeet transcriber at startup.
- If `parakeet.endpoint` is set, whisperAPI uses that endpoint directly.
- Embedded initialization failures do not stop whisperAPI startup; the Parakeet engine is disabled and Whisper remains available.
- External endpoint mode still probes at startup and fails fast if unreachable.

When `engine=parakeet` is sent, whisperAPI forwards the full audio clip using `multipart/form-data` to the Parakeet OpenAI-compatible transcription endpoint.

Endpoint behavior:

- If `parakeet.endpoint` is a base URL (for example `http://localhost:5092`), whisperAPI appends `/v1/audio/transcriptions`.
- If `parakeet.endpoint` already includes a path, that path is used as-is.

Forwarded form fields:

- `file` (uploaded audio)
- `model` (defaults to `parakeet-tdt-0.6b`)
- `language` (optional)
- `response_format=verbose_json`

Expected Parakeet response JSON (OpenAI-compatible):

```json
{
  "text": "Transcribed text content",
  "segments": []
}
```

Response:

```json
{
  "text": "Transcribed text content",
  "engine": "whisper",
  "model": "models/ggml-base.bin",
  "audio_info": {
    "format": "WAV",
    "codec": "PCM",
    "sample_rate": 16000,
    "channels": 1,
    "bit_depth": 16,
    "duration_seconds": 10.5,
    "original_size_bytes": 336000,
    "bitrate_kbps": 256
  },
  "compute_time": {
    "cpu_time_seconds": 1.23,
    "gpu_time_seconds": null
  },
  "timestamp": "2024-02-14T12:34:56Z"
}
```

See the swagger documentation for more details

### Authentication

All protected endpoints require a Bearer token:

```bash
curl -X POST http://localhost:8080/transcribe \
  -H "Authorization: Bearer your-token-here" \
  -F "audio=@sample.wav"
```

Token validation flow:

1. Check Redis cache for fast validation
2. If not in cache, check PostgreSQL database
3. If found in database, cache in Redis
4. If not found, check static tokens
5. If no match found, return 401 Unauthorized

## Testing

Run the test suite:

```bash
go test -v ./...
```

Generate test coverage:

```bash
go test -v -cover ./...
```

## Error Handling

The API returns detailed error responses:

```json
{
  "error": "Detailed error message"
}
```

Common error scenarios:

- Invalid audio format
- Unsupported codec
- File read/write errors
- Processing timeouts
- Memory limits exceeded

Authentication errors:

```json
{
  "error": "Authorization header required"
}
```

```json
{
  "error": "Invalid token"
}
```

## Monitoring

Prometheus metrics available at `/metrics`:

- `whisperapi_transcription_requests_total{status="success|error",format="wav|ogg|opus"}`
- `whisperapi_transcription_duration_seconds`
- `whisperapi_audio_duration_seconds`
- `whisperapi_memory_usage_bytes{type="allocated|system|heap"}`
- `whisperapi_cpu_time_seconds{operation="user|system|total"}`

## Contributing

1. Fork the repository
1. Create your feature branch
1. Run tests: `go test -v ./...`
1. Commit changes
1. Push to your branch
1. Create Pull Request

## Running as a Service

### FreeBSD RC Service

Create a FreeBSD service file at `/usr/local/etc/rc.d/whisperapi`:

```sh
#!/bin/sh
#
# PROVIDE: whisperapi
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="whisperapi"
rcvar="whisperapi_enable"
whisperapi_user="www"
whisperapi_group="www"
pidfile="/var/run/${name}.pid"
command="/usr/local/bin/whisperAPI"
command_args="&"

load_rc_config $name
run_rc_command "$1"
```

Make it executable and enable the service:

```bash
chmod +x /usr/local/etc/rc.d/whisperapi
echo 'whisperapi_enable="YES"' >> /etc/rc.conf
service whisperapi start
```

### Systemd Service (Linux)

Create a systemd service file at `/etc/systemd/system/whisperapi.service`:

```ini
[Unit]
Description=Whisper API Service
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
User=www-data
Group=www-data
Restart=always
RestartSec=1
WorkingDirectory=/opt/whisperapi
ExecStart=/opt/whisperapi/whisperAPI

# Security settings
PrivateTmp=true
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true
CapabilityBoundingSet=
AmbientCapabilities=

# Resource limits
LimitNOFILE=65535
MemoryMax=2G
CPUQuota=80%

[Install]
WantedBy=multi-user.target
```

Install and start the service:

```bash
# Copy application to /opt/whisperapi
sudo mkdir -p /opt/whisperapi
sudo cp whisperAPI /opt/whisperapi/
sudo cp -r models /opt/whisperapi/

# Set permissions
sudo chown -R www-data:www-data /opt/whisperapi

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable whisperapi
sudo systemctl start whisperapi

# View logs
sudo journalctl -u whisperapi -f
```

Monitor service status:

```bash
sudo systemctl status whisperapi
```

Common systemctl commands:

```bash
sudo systemctl stop whisperapi
sudo systemctl restart whisperapi
sudo systemctl disable whisperapi
```

## License

This project is licensed under the BSD 3-Clause License - see the [LICENSE](LICENSE) file for details.

Copyright (c) 2024-2025, Darcy Buskermolen <darcy@dbitech.ca>. All rights reserved.
