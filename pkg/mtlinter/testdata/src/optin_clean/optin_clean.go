// Package optinclean is a mock package for testing.
package optinclean

import (
	// Blank import to trigger opt-in.
	_ "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"
	"github.com/prometheus/client_golang/prometheus"
)

type myStruct struct {
	counter prometheus.Counter
}

func newMyStruct() *myStruct {
	return &myStruct{}
}

func (m *myStruct) foo() {
	var localCounter prometheus.Counter
	_ = localCounter

	prometheus.NewConstMetric()
}
