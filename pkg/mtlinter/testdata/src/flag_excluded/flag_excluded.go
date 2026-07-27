// Package flagexcluded is a mock package for testing.
package flagexcluded

import (
	// Blank import to trigger opt-in.
	_ "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	globalCounter    prometheus.Counter
	globalCounterVec prometheus.CounterVec
)

func init() {
	prometheus.MustRegister(globalCounter)
}
