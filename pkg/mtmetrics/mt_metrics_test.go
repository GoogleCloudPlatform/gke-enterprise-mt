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

func TestMTMetricFactory_GaugeDualEmission(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	opts := prometheus.GaugeOpts{
		Name: "shared_gauge",
		Help: "shared help",
	}
	labels := []string{"label1"}

	factoryA := NewMTMetricFactory("tenant-A", globalReg, tracker)
	gaugeA, err := factoryA.NewGaugeVec(opts, labels)
	if err != nil {
		t.Fatalf("Failed to create gauge vec A: %v", err)
	}
	gaugeA.WithLabelValues("val1").Set(10)

	// Verify global registry has value 10
	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGauge(t, globalMfs, "shared_gauge", "val1", 10)

	factoryB := NewMTMetricFactory("tenant-B", globalReg, tracker)
	gaugeB, err := factoryB.NewGaugeVec(opts, labels)
	if err != nil {
		t.Fatalf("Failed to create gauge vec B: %v", err)
	}
	gaugeB.WithLabelValues("val1").Set(20)

	// Verify global registry has sum of values (10 + 20 = 30)
	globalMfs, err = globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGauge(t, globalMfs, "shared_gauge", "val1", 30)

	// Reset gaugeA (Tenant A stops or resets)
	gaugeA.Reset()

	// Verify global registry has only Tenant B's value (20)
	globalMfs, err = globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGauge(t, globalMfs, "shared_gauge", "val1", 20)
}

func verifyStdGauge(t *testing.T, mfs []*dto_pb.MetricFamily, name string, label1Val string, expectedVal float64) {
	t.Helper()
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == name {
			for _, m := range mf.Metric {
				labelsMap := make(map[string]string)
				for _, l := range m.Label {
					labelsMap[l.GetName()] = l.GetValue()
				}
				if labelsMap["label1"] == label1Val {
					found = true
					if _, hasTenant := labelsMap["tenant_uid"]; hasTenant {
						t.Error("Expected no tenant_uid label in global registry")
					}
					if m.Gauge.GetValue() != expectedVal {
						t.Errorf("Expected gauge value %f, got %f for label %q", expectedVal, m.Gauge.GetValue(), label1Val)
					}
				}
			}
		}
	}
	if !found {
		t.Errorf("Metric %q with label %q not found", name, label1Val)
	}
}

func TestMTGauge_SetToCurrentTime(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	factoryA := NewMTMetricFactory("tenant-A", globalReg, tracker)
	gaugeA, err := factoryA.NewGaugeVec(prometheus.GaugeOpts{
		Name: "time_gauge",
		Help: "help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create gauge vec: %v", err)
	}

	g := gaugeA.WithLabelValues("val1")
	g.SetToCurrentTime()

	localReg := factoryA.(*mtMetricFactory).Registry()
	localMfs, err := localReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather local: %v", err)
	}
	verifyGaugeValueGreaterThanZero(t, localMfs, "time_gauge", "tenant-A", "val1")

	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGaugeValueGreaterThanZero(t, globalMfs, "time_gauge", "val1")
}

func TestMTGaugeVec_DeleteLabelValues(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	factoryA := NewMTMetricFactory("tenant-A", globalReg, tracker)
	gaugeA, err := factoryA.NewGaugeVec(prometheus.GaugeOpts{
		Name: "del_gauge",
		Help: "help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create gauge vec A: %v", err)
	}

	factoryB := NewMTMetricFactory("tenant-B", globalReg, tracker)
	gaugeB, err := factoryB.NewGaugeVec(prometheus.GaugeOpts{
		Name: "del_gauge",
		Help: "help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create gauge vec B: %v", err)
	}

	gaugeA.WithLabelValues("val1").Set(10)
	gaugeB.WithLabelValues("val1").Set(20)

	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGauge(t, globalMfs, "del_gauge", "val1", 30)

	deleted := gaugeA.DeleteLabelValues("val1")
	if !deleted {
		t.Error("Expected DeleteLabelValues to return true")
	}

	globalMfs, err = globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGauge(t, globalMfs, "del_gauge", "val1", 20)

	localRegA := factoryA.(*mtMetricFactory).Registry()
	localMfsA, err := localRegA.Gather()
	if err != nil {
		t.Fatalf("Failed to gather local A: %v", err)
	}
	if len(localMfsA) > 0 {
		for _, mf := range localMfsA {
			if mf.GetName() == "del_gauge" {
				for _, m := range mf.Metric {
					labelsMap := make(map[string]string)
					for _, l := range m.Label {
						labelsMap[l.GetName()] = l.GetValue()
					}
					if labelsMap["label1"] == "val1" {
						t.Error("Local gauge A still contains deleted label value 'val1'")
					}
				}
			}
		}
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
	gaugeA, err := factoryA.NewGauge(prometheus.GaugeOpts{Name: "scalar_gauge", Help: "help"})
	if err != nil {
		t.Fatalf("Failed to create gauge A: %v", err)
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
	gaugeB, err := factoryB.NewGauge(prometheus.GaugeOpts{Name: "scalar_gauge", Help: "help"})
	if err != nil {
		t.Fatalf("Failed to create gauge B: %v", err)
	}
	histB, err := factoryB.NewHistogram(prometheus.HistogramOpts{Name: "scalar_histogram", Help: "help"})
	if err != nil {
		t.Fatalf("Failed to create histogram B: %v", err)
	}

	counterA.Inc()
	counterB.Add(2)

	gaugeA.Set(10)
	gaugeB.Set(20)

	histA.Observe(1.5)
	histB.Observe(2.5)

	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdScalarMetric(t, globalMfs, "scalar_counter", 3, dto_pb.MetricType_COUNTER)
	verifyStdScalarMetric(t, globalMfs, "scalar_gauge", 30, dto_pb.MetricType_GAUGE)
	verifyStdScalarMetric(t, globalMfs, "scalar_histogram", 2, dto_pb.MetricType_HISTOGRAM) // Expected count 2

	localRegA := factoryA.(*mtMetricFactory).Registry()
	localMfsA, err := localRegA.Gather()
	if err != nil {
		t.Fatalf("Failed to gather local A: %v", err)
	}
	verifyMTScalarMetric(t, localMfsA, "scalar_counter", "tenant-A", 1, dto_pb.MetricType_COUNTER)
	verifyMTScalarMetric(t, localMfsA, "scalar_gauge", "tenant-A", 10, dto_pb.MetricType_GAUGE)
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

	// Register same name as GaugeVec -> should fail
	_, err := factory.NewGaugeVec(prometheus.GaugeOpts{
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

func verifyGaugeValueGreaterThanZero(t *testing.T, mfs []*dto_pb.MetricFamily, name string, tenantUID string, label1Val string) {
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
			if m.Gauge.GetValue() <= 0 {
				t.Errorf("Expected gauge value > 0, got %f", m.Gauge.GetValue())
			}
		}
	}
	if !found {
		t.Errorf("Metric %q not found", name)
	}
}

func verifyStdGaugeValueGreaterThanZero(t *testing.T, mfs []*dto_pb.MetricFamily, name string, label1Val string) {
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
			if m.Gauge.GetValue() <= 0 {
				t.Errorf("Expected gauge value > 0, got %f", m.Gauge.GetValue())
			}
		}
	}
	if !found {
		t.Errorf("Metric %q not found", name)
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
			case dto_pb.MetricType_GAUGE:
				if m.Gauge.GetValue() != expectedVal {
					t.Errorf("Expected gauge value %f, got %f", expectedVal, m.Gauge.GetValue())
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
			case dto_pb.MetricType_GAUGE:
				if m.Gauge.GetValue() != expectedVal {
					t.Errorf("Expected gauge value %f, got %f", expectedVal, m.Gauge.GetValue())
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

func TestMTMetricFactory_Cleanup(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	opts := prometheus.GaugeOpts{
		Name: "cleanup_gauge",
		Help: "help",
	}
	labels := []string{"label1"}

	factoryA := NewMTMetricFactory("tenant-A", globalReg, tracker)
	gaugeA, err := factoryA.NewGaugeVec(opts, labels)
	if err != nil {
		t.Fatalf("Failed to create gauge vec A: %v", err)
	}
	gaugeA.WithLabelValues("val1").Set(10)

	factoryB := NewMTMetricFactory("tenant-B", globalReg, tracker)
	gaugeB, err := factoryB.NewGaugeVec(opts, labels)
	if err != nil {
		t.Fatalf("Failed to create gauge vec B: %v", err)
	}
	gaugeB.WithLabelValues("val1").Set(20)

	// Verify global registry has sum of values (10 + 20 = 30)
	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGauge(t, globalMfs, "cleanup_gauge", "val1", 30)

	// Cleanup factoryA
	factoryA.Cleanup()

	// Verify global registry has only Tenant B's value (20)
	globalMfs, err = globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGauge(t, globalMfs, "cleanup_gauge", "val1", 20)
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

func TestMTGauge_AddSubDec(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	factory := NewMTMetricFactory("tenant-A", globalReg, tracker)
	gaugeVec, err := factory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "add_sub_gauge",
		Help: "help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create gauge vec: %v", err)
	}

	g := gaugeVec.WithLabelValues("val1")

	g.Set(10)
	g.Add(5) // 15
	g.Dec()  // 14
	g.Sub(4) // 10

	// Verify local
	localReg := factory.(*mtMetricFactory).Registry()
	localMfs, err := localReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather local: %v", err)
	}
	verifyLocalGauge(t, localMfs, "add_sub_gauge", "tenant-A", "val1", 10)

	// Verify global
	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGauge(t, globalMfs, "add_sub_gauge", "val1", 10)
}

func verifyLocalGauge(t *testing.T, mfs []*dto_pb.MetricFamily, name string, tenantUID string, label1Val string, expectedVal float64) {
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
			if m.Gauge.GetValue() != expectedVal {
				t.Errorf("Expected gauge value %f, got %f", expectedVal, m.Gauge.GetValue())
			}
		}
	}
	if !found {
		t.Errorf("Metric %q not found", name)
	}
}

func TestMTGauge_Operations(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()
	tenantUID := "tenant-A"
	factory := NewMTMetricFactory(tenantUID, globalReg, tracker)

	gauge, err := factory.NewGauge(prometheus.GaugeOpts{
		Name: "op_gauge",
		Help: "help",
	})
	if err != nil {
		t.Fatalf("Failed to create gauge: %v", err)
	}

	gauge.Set(10)
	gauge.Inc()  // 11
	gauge.Dec()  // 10
	gauge.Add(5)  // 15
	gauge.Sub(3)  // 12

	// Verify local
	localReg := factory.(*mtMetricFactory).Registry()
	localMfs, err := localReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather local: %v", err)
	}
	verifyMTScalarMetric(t, localMfs, "op_gauge", tenantUID, 12, dto_pb.MetricType_GAUGE)

	// Verify global
	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdScalarMetric(t, globalMfs, "op_gauge", 12, dto_pb.MetricType_GAUGE)
}

func TestMTGaugeVec_Operations(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()
	tenantUID := "tenant-A"
	factory := NewMTMetricFactory(tenantUID, globalReg, tracker)

	gaugeVec, err := factory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "op_gauge_vec",
		Help: "help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create gauge vec: %v", err)
	}

	g := gaugeVec.WithLabelValues("val1")
	g.Set(10)
	g.Inc()  // 11
	g.Dec()  // 10
	g.Add(5)  // 15
	g.Sub(3)  // 12

	// Verify local
	localReg := factory.(*mtMetricFactory).Registry()
	localMfs, err := localReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather local: %v", err)
	}
	verifyLocalGauge(t, localMfs, "op_gauge_vec", tenantUID, "val1", 12)

	// Verify global
	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGauge(t, globalMfs, "op_gauge_vec", "val1", 12)
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

	// Try to create a gauge vec
	gauge, err := factory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zero_init_gauge",
		Help: "help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create gauge vec: %v", err)
	}

	// This should not panic and should initialize maps
	gauge.WithLabelValues("val1").Set(10)
}

func TestSerializeDeserializeLabels(t *testing.T) {
	tests := []struct {
		name string
		lvs  []string
	}{
		{
			name: "normal labels",
			lvs:  []string{"val1", "val2"},
		},
		{
			name: "empty label",
			lvs:  []string{""},
		},
		{
			name: "multiple empty labels",
			lvs:  []string{"", ""},
		},
		{
			name: "mixed empty labels",
			lvs:  []string{"val1", "", "val2"},
		},
		{
			name: "empty slice",
			lvs:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serialized := serializeLabels(tt.lvs)
			deserialized := deserializeLabels(serialized)
			if len(deserialized) != len(tt.lvs) {
				t.Fatalf("Expected length %d, got %d. Serialized key: %q, Deserialized: %v", len(tt.lvs), len(deserialized), serialized, deserialized)
			}
			for i, v := range deserialized {
				if v != tt.lvs[i] {
					t.Errorf("Expected element %d to be %q, got %q", i, tt.lvs[i], v)
				}
			}
		})
	}
}

func TestMTGaugeVec_DeleteEmptyLabel(t *testing.T) {
	globalReg := prometheus.NewRegistry()
	tracker := NewGlobalMetricsTracker()

	factory := NewMTMetricFactory("tenant-A", globalReg, tracker)
	gaugeVec, err := factory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "empty_label_gauge",
		Help: "help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create gauge vec: %v", err)
	}

	// Set value with empty label
	gaugeVec.WithLabelValues("").Set(10)

	// Verify global registry has value 10
	globalMfs, err := globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	verifyStdGauge(t, globalMfs, "empty_label_gauge", "", 10)

	// Delete label value (this used to panic or fail to delete because of deserialization bug)
	// It should return true indicating it was deleted.
	deleted := gaugeVec.DeleteLabelValues("")
	if !deleted {
		t.Error("Expected DeleteLabelValues to return true")
	}

	// Verify global registry is empty or doesn't have the metric value
	globalMfs, err = globalReg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather global: %v", err)
	}
	for _, mf := range globalMfs {
		if mf.GetName() == "empty_label_gauge" {
			for _, m := range mf.Metric {
				labelsMap := make(map[string]string)
				for _, l := range m.Label {
					labelsMap[l.GetName()] = l.GetValue()
				}
				if labelsMap["label1"] == "" {
					t.Error("Global gauge still contains deleted empty label value")
				}
			}
		}
	}
}

func TestMTGauge_AggregationStrategies(t *testing.T) {
	opts := prometheus.GaugeOpts{
		Name: "strategy_gauge",
		Help: "help",
	}
	labels := []string{"label1"}

	tests := []struct {
		name     string
		strategy AggregationStrategy
		vals     map[string]float64 // tenant -> val
		expected float64
	}{
		{
			name:     "default sum strategy",
			strategy: StrategySum,
			vals:     map[string]float64{"tenant-A": 10, "tenant-B": 20},
			expected: 30,
		},
		{
			name:     "max strategy",
			strategy: StrategyMax,
			vals:     map[string]float64{"tenant-A": 10, "tenant-B": 20, "tenant-C": 15},
			expected: 20,
		},
		{
			name:     "min strategy",
			strategy: StrategyMin,
			vals:     map[string]float64{"tenant-A": 10, "tenant-B": 20, "tenant-C": 5},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalReg := prometheus.NewRegistry()
			tracker := NewGlobalMetricsTracker()
			tracker.SetAggregationStrategy("strategy_gauge", tt.strategy)

			for tenant, val := range tt.vals {
				factory := NewMTMetricFactory(tenant, globalReg, tracker)
				gauge, err := factory.NewGaugeVec(opts, labels)
				if err != nil {
					t.Fatalf("Failed to create gauge vec for %s: %v", tenant, err)
				}
				gauge.WithLabelValues("val1").Set(val)
			}

			globalMfs, err := globalReg.Gather()
			if err != nil {
				t.Fatalf("Failed to gather global: %v", err)
			}
			verifyStdGauge(t, globalMfs, "strategy_gauge", "val1", tt.expected)
		})
	}
}

func TestMTGauge_ScalarAggregationStrategies(t *testing.T) {
	opts := prometheus.GaugeOpts{
		Name: "strategy_scalar_gauge",
		Help: "help",
	}

	tests := []struct {
		name     string
		strategy AggregationStrategy
		vals     map[string]float64 // tenant -> val
		expected float64
	}{
		{
			name:     "default sum strategy",
			strategy: StrategySum,
			vals:     map[string]float64{"tenant-A": 10, "tenant-B": 20},
			expected: 30,
		},
		{
			name:     "max strategy",
			strategy: StrategyMax,
			vals:     map[string]float64{"tenant-A": 10, "tenant-B": 20, "tenant-C": 15},
			expected: 20,
		},
		{
			name:     "min strategy",
			strategy: StrategyMin,
			vals:     map[string]float64{"tenant-A": 10, "tenant-B": 20, "tenant-C": 5},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalReg := prometheus.NewRegistry()
			tracker := NewGlobalMetricsTracker()
			tracker.SetAggregationStrategy("strategy_scalar_gauge", tt.strategy)

			for tenant, val := range tt.vals {
				factory := NewMTMetricFactory(tenant, globalReg, tracker)
				gauge, err := factory.NewGauge(opts)
				if err != nil {
					t.Fatalf("Failed to create gauge for %s: %v", tenant, err)
				}
				gauge.Set(val)
			}

			globalMfs, err := globalReg.Gather()
			if err != nil {
				t.Fatalf("Failed to gather global: %v", err)
			}
			verifyStdScalarMetric(t, globalMfs, "strategy_scalar_gauge", tt.expected, dto_pb.MetricType_GAUGE)
		})
	}
}



