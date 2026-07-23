/*
 * Copyright 2026 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mtmetrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto_pb "github.com/prometheus/client_model/go"
)

func TestMTMetricFactory(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()
	tenantUID := "tenant-A"
	factory := NewMTMetricFactory(tenantUID, globalReg, tracker)

	counter, err := factory.NewCounterVec(prometheus.CounterOpts{
		Name: "test_counter_mt",
		Help: "test help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create counter vec: %v", err)
	}

	counter.WithLabelValues("val1").Inc()

	// Verify local registry
	localReg := factory.(*mtMetricFactory).Registry()
	localMfs, err := localReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather local: %v", err)
	}

	verifyMTMetric(t, localMfs, "test_counter_mt", tenantUID, "val1", 1)

	// Verify global registry
	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}

	verifyStdMetric(t, globalMfs, "test_counter_mt", "val1", 1)
}

func TestMTMetricFactory_DualEmission(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	opts := prometheus.CounterOpts{
		Name: "shared_counter",
		Help: "shared help",
	}
	labels := []string{"label1"}

	factoryA := NewMTMetricFactory("tenant-A", globalReg, tracker)
	counterA, err := factoryA.NewCounterVec(opts, labels)
	if err != nil {
		t.Fatalf("Failed to create counter vec A: %v", err)
	}
	counterA.WithLabelValues("val1").Inc()

	// This should not panic because of tracker
	factoryB := NewMTMetricFactory("tenant-B", globalReg, tracker)
	counterB, err := factoryB.NewCounterVec(opts, labels)
	if err != nil {
		t.Fatalf("Failed to create counter vec B: %v", err)
	}
	counterB.WithLabelValues("val1").Add(2)

	// Verify global registry has one metric with value 3 (1 from tenant-A + 2 from tenant-B)
	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}

	verifyStdMetric(t, globalMfs, "shared_counter", "val1", 3)
}

func verifyMTMetric(t *testing.T, mfs []*dto_pb.MetricFamily, name string, tenantUID string, label1Val string, expectedVal float64) {
	t.Helper()
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == name {
			found = true
			if len(mf.Metric) != 1 {
				t.Fatalf("Expected 1 metric, got %d", len(mf.Metric))
			}
			m := mf.Metric[0]
			labelsMap := make(map[string]string)
			for _, l := range m.Label {
				labelsMap[l.GetName()] = l.GetValue()
			}

			if labelsMap["tenant_uid"] != tenantUID {
				t.Errorf("Expected tenant_uid %q, got %q", tenantUID, labelsMap["tenant_uid"])
			}
			if labelsMap["label1"] != label1Val {
				t.Errorf("Expected label1 %q, got %q", label1Val, labelsMap["label1"])
			}
			if m.Counter.GetValue() != expectedVal {
				t.Errorf("Expected counter value %f, got %f", expectedVal, m.Counter.GetValue())
			}
		}
	}
	if !found {
		t.Errorf("Metric %q not found", name)
	}
}

func verifyStdMetric(t *testing.T, mfs []*dto_pb.MetricFamily, name string, label1Val string, expectedVal float64) {
	t.Helper()
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == name {
			found = true
			if len(mf.Metric) != 1 {
				t.Fatalf("Expected 1 metric, got %d", len(mf.Metric))
			}
			m := mf.Metric[0]
			labelsMap := make(map[string]string)
			for _, l := range m.Label {
				labelsMap[l.GetName()] = l.GetValue()
			}
			if _, hasTenant := labelsMap["tenant_uid"]; hasTenant {
				t.Error("Expected no tenant_uid label in global registry")
			}
			if labelsMap["label1"] != label1Val {
				t.Errorf("Expected label1 %q, got %q", label1Val, labelsMap["label1"])
			}
			if m.Counter.GetValue() != expectedVal {
				t.Errorf("Expected counter value %f, got %f", expectedVal, m.Counter.GetValue())
			}
		}
	}
	if !found {
		t.Errorf("Metric %q not found", name)
	}
}

func TestScalarMetrics(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	factoryA := NewMTMetricFactory("tenant-A", globalReg, tracker)
	counterA, err := factoryA.NewCounter(prometheus.CounterOpts{Name: "scalar_counter", Help: "help"})
	if err != nil {
		t.Fatalf("Failed to create counter A: %v", err)
	}
	histA, err := factoryA.NewHistogram(prometheus.HistogramOpts{Name: "scalar_histogram", Help: "help"})
	if err != nil {
		t.Fatalf("Failed to create histogram A: %v", err)
	}

	factoryB := NewMTMetricFactory("tenant-B", globalReg, tracker)
	counterB, err := factoryB.NewCounter(prometheus.CounterOpts{Name: "scalar_counter", Help: "help"})
	if err != nil {
		t.Fatalf("Failed to create counter B: %v", err)
	}
	histB, err := factoryB.NewHistogram(prometheus.HistogramOpts{Name: "scalar_histogram", Help: "help"})
	if err != nil {
		t.Fatalf("Failed to create histogram B: %v", err)
	}

	counterA.Inc()
	counterB.Add(2)

	histA.Observe(1.5)
	histB.Observe(2.5)

	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdScalarMetric(t, globalMfs, "scalar_counter", 3, dto_pb.MetricType_COUNTER)
	verifyStdScalarMetric(t, globalMfs, "scalar_histogram", 2, dto_pb.MetricType_HISTOGRAM) // Expected count 2

	localRegA := factoryA.(*mtMetricFactory).Registry()
	localMfsA, err := localRegA.Gather()
	if err != nil {
		t.Fatalf("Failed to gather local A: %v", err)
	}
	verifyMTScalarMetric(t, localMfsA, "scalar_counter", "tenant-A", 1, dto_pb.MetricType_COUNTER)
	verifyMTScalarMetric(t, localMfsA, "scalar_histogram", "tenant-A", 1, dto_pb.MetricType_HISTOGRAM)
}

func TestRegistrationErrorPropagation(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()
	factory := NewMTMetricFactory("tenant-A", globalReg, tracker)

	opts := prometheus.CounterOpts{
		Name: "conflict_metric",
		Help: "help",
	}
	if _, err := factory.NewCounterVec(opts, []string{"label1"}); err != nil {
		t.Fatalf("Failed to create counter vec: %v", err)
	}

	// Register same name as HistogramVec -> should fail
	_, err := factory.NewHistogramVec(prometheus.HistogramOpts{
		Name: "conflict_metric",
		Help: "help",
	}, []string{"label1"})
	if err == nil {
		t.Error("Expected error when registering duplicate metric with different type, got nil")
	}
}

func TestLabelConsistencyValidation(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()
	factory := NewMTMetricFactory("tenant-A", globalReg, tracker)

	opts := prometheus.CounterOpts{
		Name: "label_check_metric",
		Help: "help",
	}
	if _, err := factory.NewCounterVec(opts, []string{"label1"}); err != nil {
		t.Fatalf("Failed to create counter vec: %v", err)
	}

	// Different label count
	_, err := factory.NewCounterVec(opts, []string{"label1", "label2"})
	if err == nil {
		t.Error("Expected error when registering duplicate metric with different label count, got nil")
	}

	// Different label names
	_, err = factory.NewCounterVec(opts, []string{"different_label"})
	if err == nil {
		t.Error("Expected error when registering duplicate metric with different label names, got nil")
	}
}

func verifyStdScalarMetric(t *testing.T, mfs []*dto_pb.MetricFamily, name string, expectedVal float64, metricType dto_pb.MetricType) {
	t.Helper()
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == name {
			found = true
			if len(mf.Metric) != 1 {
				t.Fatalf("Expected 1 metric, got %d", len(mf.Metric))
			}
			m := mf.Metric[0]
			if len(m.Label) != 0 {
				t.Errorf("Expected 0 labels for scalar metric, got %d", len(m.Label))
			}
			switch metricType {
			case dto_pb.MetricType_COUNTER:
				if m.Counter.GetValue() != expectedVal {
					t.Errorf("Expected counter value %f, got %f", expectedVal, m.Counter.GetValue())
				}
			case dto_pb.MetricType_HISTOGRAM:
				if m.Histogram.GetSampleCount() != uint64(expectedVal) {
					t.Errorf("Expected histogram sample count %d, got %d", uint64(expectedVal), m.Histogram.GetSampleCount())
				}
			}
		}
	}
	if !found {
		t.Errorf("Metric %q not found", name)
	}
}

func verifyMTScalarMetric(t *testing.T, mfs []*dto_pb.MetricFamily, name string, tenantUID string, expectedVal float64, metricType dto_pb.MetricType) {
	t.Helper()
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == name {
			found = true
			if len(mf.Metric) != 1 {
				t.Fatalf("Expected 1 metric, got %d", len(mf.Metric))
			}
			m := mf.Metric[0]
			labelsMap := make(map[string]string)
			for _, l := range m.Label {
				labelsMap[l.GetName()] = l.GetValue()
			}
			if len(labelsMap) != 1 || labelsMap["tenant_uid"] != tenantUID {
				t.Errorf("Expected only tenant_uid label %q, got %v", tenantUID, labelsMap)
			}
			switch metricType {
			case dto_pb.MetricType_COUNTER:
				if m.Counter.GetValue() != expectedVal {
					t.Errorf("Expected counter value %f, got %f", expectedVal, m.Counter.GetValue())
				}
			case dto_pb.MetricType_HISTOGRAM:
				if m.Histogram.GetSampleCount() != uint64(expectedVal) {
					t.Errorf("Expected histogram sample count %d, got %d", uint64(expectedVal), m.Histogram.GetSampleCount())
				}
			}
		}
	}
	if !found {
		t.Errorf("Metric %q not found", name)
	}
}

func TestMTCounterVec_ResetDoesNotClearGlobal(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	opts := prometheus.CounterOpts{
		Name: "test_reset_counter",
		Help: "help",
	}
	labels := []string{"label1"}

	factory := NewMTMetricFactory("tenant-A", globalReg, tracker)
	counter, err := factory.NewCounterVec(opts, labels)
	if err != nil {
		t.Fatalf("Failed to create counter vec: %v", err)
	}
	counter.WithLabelValues("val1").Inc()

	// Verify global registry has value 1
	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdMetric(t, globalMfs, "test_reset_counter", "val1", 1)

	// Reset the counter vector (should only affect local)
	counter.Reset()

	// Verify global registry STILL has value 1
	globalMfs, err = globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdMetric(t, globalMfs, "test_reset_counter", "val1", 1)

	// Verify local registry IS cleared (reset)
	localReg := factory.(*mtMetricFactory).Registry()
	localMfs, err := localReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather local: %v", err)
	}
	for _, mf := range localMfs {
		if mf.GetName() == "test_reset_counter" {
			if len(mf.Metric) != 0 {
				t.Errorf("Expected local counter to be reset (0 metrics), got %d", len(mf.Metric))
			}
		}
	}
}

func TestMTObserverVec_ResetDoesNotClearGlobal(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	opts := prometheus.HistogramOpts{
		Name: "test_reset_histogram",
		Help: "help",
	}
	labels := []string{"label1"}

	factory := NewMTMetricFactory("tenant-A", globalReg, tracker)
	histVec, err := factory.NewHistogramVec(opts, labels)
	if err != nil {
		t.Fatalf("Failed to create histogram vec: %v", err)
	}
	histVec.WithLabelValues("val1").Observe(1.5)

	// Verify global registry has count 1
	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdHistogram(t, globalMfs, "test_reset_histogram", "label1", "val1", 1)

	// Reset the histogram vector
	histVec.Reset()

	// Verify global registry STILL has count 1
	globalMfs, err = globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdHistogram(t, globalMfs, "test_reset_histogram", "label1", "val1", 1)
}

func verifyStdHistogram(t *testing.T, mfs []*dto_pb.MetricFamily, name string, labelName, labelVal string, expectedCount uint64) {
	t.Helper()
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == name {
			for _, m := range mf.Metric {
				labelsMap := make(map[string]string)
				for _, l := range m.Label {
					labelsMap[l.GetName()] = l.GetValue()
				}
				if labelsMap[labelName] == labelVal {
					found = true
					if _, hasTenant := labelsMap["tenant_uid"]; hasTenant {
						t.Error("Expected no tenant_uid label in global registry")
					}
					if m.Histogram.GetSampleCount() != expectedCount {
						t.Errorf("Expected histogram count %d, got %d for label %q", expectedCount, m.Histogram.GetSampleCount(), labelVal)
					}
				}
			}
		}
	}
	if !found {
		t.Errorf("Metric %q with label %q not found", name, labelVal)
	}
}

func TestMultiGatherer_ScalarCollision(t *testing.T) {
	mg := NewMultiGatherer()
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	factoryA := NewMTMetricFactory("tenant-A", globalReg, tracker)
	if _, err := factoryA.NewCounter(prometheus.CounterOpts{Name: "shared_scalar", Help: "help"}); err != nil {
		t.Fatalf("Failed to create counter A: %v", err)
	}

	factoryB := NewMTMetricFactory("tenant-B", globalReg, tracker)
	if _, err := factoryB.NewCounter(prometheus.CounterOpts{Name: "shared_scalar", Help: "help"}); err != nil {
		t.Fatalf("Failed to create counter B: %v", err)
	}

	if err := mg.Register("tenant-A", factoryA.(*mtMetricFactory).Registry()); err != nil {
		t.Fatalf("Failed to register tenant-A: %v", err)
	}
	if err := mg.Register("tenant-B", factoryB.(*mtMetricFactory).Registry()); err != nil {
		t.Fatalf("Failed to register tenant-B: %v", err)
	}

	// Gather should merge both tenant's local registry where they both have "shared_scalar"
	// This will collide if local scalar metrics do not have "tenant_uid" label.
	mfs, err := mg.Gather()
	if err != nil {
		t.Fatalf("Gather failed (possible label collision on scalar metrics): %v", err)
	}

	if len(mfs) != 1 {
		t.Fatalf("Expected 1 metric family, got %d", len(mfs))
	}
	mf := mfs[0]
	if len(mf.Metric) != 2 {
		t.Fatalf("Expected 2 metrics in merged family, got %d", len(mf.Metric))
	}
}

func TestTracker_GlobalRegistryConflict(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	// Register directly to globalReg as GaugeVec
	directGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "conflict_metric",
		Help: "help",
	}, []string{"label1"})
	if err := globalReg.Register(directGauge); err != nil {
		t.Fatalf("Failed to register direct gauge: %v", err)
	}

	factory := NewMTMetricFactory("tenant-A", globalReg, tracker)

	// Try to create CounterVec with same name via factory.
	_, err := factory.NewCounterVec(prometheus.CounterOpts{
		Name: "conflict_metric",
		Help: "help",
	}, []string{"label1"})

	if err == nil {
		t.Error("Expected error when global registry has conflict metric of different type, got nil")
	}
}

func TestTracker_GlobalRegistryBypass(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	// Register directly to globalReg as CounterVec
	directCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bypass_counter",
		Help: "help",
	}, []string{"label1"})
	if err := globalReg.Register(directCounter); err != nil {
		t.Fatalf("Failed to register direct counter: %v", err)
	}

	factory := NewMTMetricFactory("tenant-A", globalReg, tracker)

	// Create via factory. Should succeed by finding the existing one.
	c, err := factory.NewCounterVec(prometheus.CounterOpts{
		Name: "bypass_counter",
		Help: "help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Expected success when types match, got: %v", err)
	}

	c.WithLabelValues("val1").Inc()

	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdMetric(t, globalMfs, "bypass_counter", "val1", 1)
}

func TestGlobalMetricsTracker_ZeroValueInit(t *testing.T) {
	tracker := &GlobalMetricsTracker{}
	globalReg := prometheus.NewRegistry()

	// Try to create a counter vec using a factory with this zero-initialized tracker.
	factory := NewMTMetricFactory("tenant-A", globalReg, tracker)
	counter, err := factory.NewCounterVec(prometheus.CounterOpts{
		Name: "zero_init_counter",
		Help: "help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create counter vec: %v", err)
	}

	// This should not panic
	counter.WithLabelValues("val1").Inc()
}
