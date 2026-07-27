// Package pkg2 is a mock package for testing.
package pkg2

import (
	// Blank import to trigger opt-in.
	_ "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	globalCounter prometheus.Counter
)

func init() {
	prometheus.MustRegister(globalCounter)
}
