// Package promauto is a mock package for testing.
package promauto

import "github.com/prometheus/client_golang/prometheus"

// NewCounter is a mock.
func NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	return nil
}

// NewCounterVec is a mock.
func NewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec {
	return nil
}
