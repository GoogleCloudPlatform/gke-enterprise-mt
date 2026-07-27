// Package optinignored is a mock package for testing.
package optinignored

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	globalCounter    prometheus.Counter
	globalCounterVec prometheus.CounterVec
)

func init() {
	prometheus.MustRegister(globalCounter)
}
