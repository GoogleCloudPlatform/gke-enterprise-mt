// Package prometheus is a mock package for testing.
package prometheus

// Counter is a mock.
type Counter any

// CounterVec is a mock.
type CounterVec struct{}

// CounterOpts is a mock.
type CounterOpts struct{ Name string }

// Gauge is a mock.
type Gauge any

// GaugeVec is a mock.
type GaugeVec struct{}

// MustRegister is a mock.
func MustRegister(cs ...any) {}

// Register is a mock.
func Register(c any) error { return nil }

// NewConstMetric is a mock.
func NewConstMetric() {}

// Registerer is a mock.
type Registerer struct{}

// Register is a mock.
func (Registerer) Register(c any) error { return nil }

// MustRegister is a mock.
func (Registerer) MustRegister(cs ...any) {}

// DefaultRegisterer is a mock.
var DefaultRegisterer Registerer
