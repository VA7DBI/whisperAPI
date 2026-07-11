// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPostParakeetJSON_SetsExplicitContentLength(t *testing.T) {
	payload := []byte(`{"audio_base64":"abc","sample_rate":16000}`)

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

	resp, err := postParakeetJSON(ts.URL, payload, 2*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, http.MethodPost, seenMethod)
	assert.Equal(t, int64(len(payload)), seenContentLength)
	assert.Empty(t, seenTransferEncoding)
	assert.Equal(t, payload, seenBody)
}
