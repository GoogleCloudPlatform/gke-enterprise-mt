// Package optinviolation is a mock package for testing.
package optinviolation

import (
	// Blank import to trigger opt-in.
	_ "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"
	"github.com/prometheus/client_golang/prometheus"
	_ "github.com/prometheus/client_golang/prometheus/promauto" // want "import of promauto is forbidden in MT mode; it registers metrics globally"
)

type myStruct struct {
	c prometheus.Counter
}

var (
	globalCounter       prometheus.Counter            // want "package-level global metric variable is forbidden in MT mode: globalCounter"
	globalCounterVec    prometheus.CounterVec         // want "package-level global metric variable is forbidden in MT mode: globalCounterVec"
	globalCounterPtr    *prometheus.Counter           // want "package-level global metric variable is forbidden in MT mode: globalCounterPtr"
	globalCounterPtrPtr **prometheus.Counter          // want "package-level global metric variable is forbidden in MT mode: globalCounterPtrPtr"
	globalSlice         []prometheus.Counter          // want "package-level global metric variable is forbidden in MT mode: globalSlice"
	globalMap           map[string]prometheus.Counter // want "package-level global metric variable is forbidden in MT mode: globalMap"
	globalStruct        myStruct                      // want "package-level global metric variable is forbidden in MT mode: globalStruct"
)

func init() {
	prometheus.MustRegister(globalCounter) // want "direct call to prometheus.MustRegister is forbidden; use mtmetrics factory instead"
	prometheus.Register(globalCounter)     // want "direct call to prometheus.Register is forbidden; use mtmetrics factory instead"
}
