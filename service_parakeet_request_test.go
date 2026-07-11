// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VA7DBI/whisperAPI/config"
	"github.com/stretchr/testify/assert"
)

func TestResolveParakeetTranscriptionEndpoint_AppendsDefaultPath(t *testing.T) {
	resolved, err := resolveParakeetTranscriptionEndpoint("http://localhost:5092")
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:5092/v1/audio/transcriptions", resolved)
}

func TestResolveParakeetTranscriptionEndpoint_UsesProvidedPath(t *testing.T) {
	resolved, err := resolveParakeetTranscriptionEndpoint("http://localhost:5092/custom/transcribe")
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:5092/custom/transcribe", resolved)
}

func TestPostParakeetMultipart_SetsExplicitContentLength(t *testing.T) {
	payload := []byte("test-payload")

	var seenMethod string
	var seenContentLength int64
	var seenTransferEncoding []string
	var seenBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenContentLength = r.ContentLength
		seenTransferEncoding = r.TransferEncoding
		seenBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"ok"}`))
	}))
	defer ts.Close()

	resp, err := postParakeetMultipart(ts.URL, "application/octet-stream", payload, 2*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, http.MethodPost, seenMethod)
	assert.Equal(t, int64(len(payload)), seenContentLength)
	assert.Empty(t, seenTransferEncoding)
	assert.Equal(t, payload, seenBody)
}

func TestTranscribeWithParakeet_SendsMultipartFileRequest(t *testing.T) {
	tmp, err := os.CreateTemp("", "parakeet-audio-*.wav")
	assert.NoError(t, err)
	defer os.Remove(tmp.Name())

	_, err = tmp.Write([]byte("RIFFfake"))
	assert.NoError(t, err)
	assert.NoError(t, tmp.Close())

	var gotModel, gotLanguage, gotResponseFormat string
	var gotFilename string
	var gotFileBytes []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaType, params, parseErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
		assert.NoError(t, parseErr)
		assert.Equal(t, "multipart/form-data", mediaType)

		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, partErr := mr.NextPart()
			if partErr == io.EOF {
				break
			}
			assert.NoError(t, partErr)

			name := part.FormName()
			switch name {
			case "file":
				gotFilename = filepath.Base(part.FileName())
				gotFileBytes, _ = io.ReadAll(part)
			case "model":
				b, _ := io.ReadAll(part)
				gotModel = string(b)
			case "language":
				b, _ := io.ReadAll(part)
				gotLanguage = string(b)
			case "response_format":
				b, _ := io.ReadAll(part)
				gotResponseFormat = string(b)
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"ok","segments":[]}`))
	}))
	defer ts.Close()

	s := &TranscriptionService{}
	s.config = &config.Config{}
	s.config.Parakeet.Endpoint = ts.URL
	s.config.Parakeet.Language = "en"
	s.config.Parakeet.Model = "parakeet-tdt-0.6b"
	s.config.Parakeet.TimeoutSeconds = 5

	text, segments, confidence, err := s.transcribeWithParakeet(tmp.Name())
	assert.NoError(t, err)
	assert.Equal(t, "ok", text)
	assert.Empty(t, segments)
	assert.Equal(t, float64(0), confidence)

	assert.Equal(t, filepath.Base(tmp.Name()), gotFilename)
	assert.Equal(t, []byte("RIFFfake"), gotFileBytes)
	assert.Equal(t, "parakeet-tdt-0.6b", gotModel)
	assert.Equal(t, "en", gotLanguage)
	assert.Equal(t, "verbose_json", gotResponseFormat)
}
