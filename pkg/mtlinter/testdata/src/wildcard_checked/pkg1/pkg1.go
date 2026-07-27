// Package pkg1 is a mock package for testing.
package pkg1

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	globalCounter prometheus.Counter // want "package-level global metric variable is forbidden in MT mode: globalCounter"
)

func init() {
	prometheus.MustRegister(globalCounter) // want "direct call to prometheus.MustRegister is forbidden; use mtmetrics factory instead"
}
