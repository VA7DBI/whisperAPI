// Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
// SPDX-License-Identifier: BSD-3-Clause

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TranscriptionRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "whisperapi_transcription_requests_total",
		Help: "Total number of transcription requests",
	}, []string{"status", "format"})

	TranscriptionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "whisperapi_transcription_duration_seconds",
		Help:    "Time spent processing transcription requests",
		Buckets: prometheus.ExponentialBuckets(0.1, 2.0, 10), // 0.1s to ~51.2s
	}, []string{"format"})

	AudioDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "whisperapi_audio_duration_seconds",
		Help:    "Duration of processed audio files",
		Buckets: prometheus.ExponentialBuckets(1, 2.0, 10), // 1s to ~512s
	}, []string{"format"})

	MemoryUsage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "whisperapi_memory_usage_bytes",
		Help: "Memory usage during transcription",
	}, []string{"type"})

	CPUTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "whisperapi_cpu_time_seconds",
		Help:    "CPU time spent on transcription",
		Buckets: prometheus.ExponentialBuckets(0.1, 2.0, 10),
	}, []string{"operation"})

	GPUTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "whisperapi_gpu_time_seconds",
		Help:    "GPU time spent on transcription (if available)",
		Buckets: prometheus.ExponentialBuckets(0.1, 2.0, 10),
	}, []string{"operation"})
)
