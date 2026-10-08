// Package optinclean is a mock package for testing.
package optinclean

import (
	"github.com/prometheus/client_golang/prometheus"
	_ "github.com/prometheus/client_golang/prometheus/promauto"
)

var testGlobalCounter prometheus.Counter

func init() {
	prometheus.MustRegister(testGlobalCounter)
	prometheus.DefaultRegisterer.MustRegister(testGlobalCounter)
}
