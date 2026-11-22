package framework

import (
	"fmt"
	"strings"

	"k8s.io/klog/v2"
	"github.com/prometheus/client_golang/prometheus"
	mtmetrics "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"
)

const (
	providerConfigControllerRunningMetricName = "providerconfig_controller_running"
	controllerLabelName                       = "controller"
	finalizerCleanupSuffix                    = "-cleanup"
)

// controllerNameFromFinalizer extracts the controller name from a finalizer
// matching the pattern "<domain>/<controller>-cleanup" (for example,
// "multiproject.networking.gke.io/neg-cleanup" -> "neg").
// It returns an error if finalizerName does not match the expected pattern.
func controllerNameFromFinalizer(finalizerName string) (string, error) {
	domain, name, hasSlash := strings.Cut(finalizerName, "/")
	if !hasSlash || domain == "" || !strings.HasSuffix(name, finalizerCleanupSuffix) {
		return "", fmt.Errorf("finalizer %q does not match expected pattern '<domain>/<controller>-cleanup'", finalizerName)
	}
	controllerName := strings.TrimSuffix(name, finalizerCleanupSuffix)
	if controllerName == "" || strings.Contains(controllerName, "/") {
		return "", fmt.Errorf("finalizer %q does not match expected pattern '<domain>/<controller>-cleanup'", finalizerName)
	}
	return controllerName, nil
}

func (m *manager) resolveControllerName() (string, error) {
	if m.controllerName != "" {
		return m.controllerName, nil
	}
	return controllerNameFromFinalizer(m.finalizerName)
}

func (m *manager) setControllerRunningMetric(tenantUID string, val float64) {
	if tenantUID == "" {
		return
	}
	ctrlName, err := m.resolveControllerName()
	if err != nil {
		klog.Errorf("cannot emit %s metric for tenant %s: %v", providerConfigControllerRunningMetricName, tenantUID, err)
		return
	}
	opts := prometheus.GaugeOpts{
		Name: providerConfigControllerRunningMetricName,
		Help: "Indicates whether the ProviderConfig controller for a tenant is running (1 = running, 0 = startup failed).",
	}
	factory := mtmetrics.GetOrCreateFactory(tenantUID)
	gaugeVec, err := factory.NewGaugeVec(opts, []string{controllerLabelName})
	if err != nil {
		klog.Errorf("failed to register %s metric for tenant %s: %v", providerConfigControllerRunningMetricName, tenantUID, err)
		return
	}
	gaugeVec.WithLabelValues(ctrlName).Set(val)
}
