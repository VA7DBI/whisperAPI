// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VA7DBI/whisperAPI/config"
	"github.com/stretchr/testify/assert"
)

func TestProbeParakeetEndpoint_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	err := probeParakeetEndpoint(ts.URL, 2*time.Second)
	assert.NoError(t, err)
}

func TestProbeParakeetEndpoint_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	err := probeParakeetEndpoint(ts.URL, 2*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 503")
}

func TestValidateConfiguredParakeetEndpoint_EmptyEndpoint(t *testing.T) {
	cfg := &config.Config{}

	err := validateConfiguredParakeetEndpoint(cfg)
	assert.NoError(t, err)
}

func TestValidateConfiguredParakeetEndpoint_InvalidEndpoint(t *testing.T) {
	cfg := &config.Config{}
	cfg.Parakeet.Endpoint = "://bad-url"
	cfg.Parakeet.TimeoutSeconds = 1

	err := validateConfiguredParakeetEndpoint(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "startup probe failed")
}
