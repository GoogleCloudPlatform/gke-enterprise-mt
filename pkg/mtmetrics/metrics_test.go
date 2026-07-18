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

func TestStdMetricFactory(t *testing.T) {
	reg := prometheus.NewRegistry()
	factory := NewStdMetricFactory(reg)

	counter, err := factory.NewCounterVec(prometheus.CounterOpts{
		Name: "test_counter",
		Help: "test help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create counter: %v", err)
	}

	counter.WithLabelValues("val1").Inc()

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather: %v", err)
	}

	if len(mfs) != 1 {
		t.Fatalf("Expected 1 metric family, got %d", len(mfs))
	}

	mf := mfs[0]
	if mf.GetName() != "test_counter" {
		t.Errorf("Expected metric name 'test_counter', got %q", mf.GetName())
	}

	if len(mf.Metric) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(mf.Metric))
	}

	m := mf.Metric[0]
	if len(m.Label) != 1 {
		t.Fatalf("Expected 1 label, got %d", len(m.Label))
	}

	if m.Label[0].GetName() != "label1" || m.Label[0].GetValue() != "val1" {
		t.Errorf("Unexpected label: %s=%s", m.Label[0].GetName(), m.Label[0].GetValue())
	}

	if m.Counter.GetValue() != 1 {
		t.Errorf("Expected counter value 1, got %f", m.Counter.GetValue())
	}
}

func TestStdMetricFactory_Cleanup(t *testing.T) {
	reg := prometheus.NewRegistry()
	factory := NewStdMetricFactory(reg)
	// Should not panic
	factory.Cleanup()
}

func TestStdMetricFactory_OtherMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	factory := NewStdMetricFactory(reg)

	// Test GaugeVec
	gaugeVec, err := factory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_gauge_vec",
		Help: "test help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create gauge vec: %v", err)
	}
	gaugeVec.WithLabelValues("val1").Set(5)

	// Test HistogramVec (ObserverVec)
	histVec, err := factory.NewHistogramVec(prometheus.HistogramOpts{
		Name: "test_hist_vec",
		Help: "test help",
	}, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to create hist vec: %v", err)
	}
	histVec.WithLabelValues("val1").Observe(1.5)

	// Test Counter
	counter, err := factory.NewCounter(prometheus.CounterOpts{
		Name: "test_counter_single",
		Help: "test help",
	})
	if err != nil {
		t.Fatalf("Failed to create counter: %v", err)
	}
	counter.Inc()

	// Test Gauge
	gauge, err := factory.NewGauge(prometheus.GaugeOpts{
		Name: "test_gauge_single",
		Help: "test help",
	})
	if err != nil {
		t.Fatalf("Failed to create gauge: %v", err)
	}
	gauge.Set(10)

	// Test Histogram
	hist, err := factory.NewHistogram(prometheus.HistogramOpts{
		Name: "test_hist_single",
		Help: "test help",
	})
	if err != nil {
		t.Fatalf("Failed to create histogram: %v", err)
	}
	hist.Observe(2.5)

	// Gather and verify
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather: %v", err)
	}

	// We expect 5 metric families (gaugeVec, histVec, counter, gauge, hist)
	if len(mfs) != 5 {
		t.Fatalf("Expected 5 metric families, got %d", len(mfs))
	}

	// Map for easy lookup
	familyMap := make(map[string]*dto_pb.MetricFamily)
	for _, mf := range mfs {
		familyMap[mf.GetName()] = mf
	}

	// Verify GaugeVec
	if mf, ok := familyMap["test_gauge_vec"]; !ok {
		t.Error("Missing test_gauge_vec")
	} else {
		if len(mf.Metric) != 1 || mf.Metric[0].Gauge.GetValue() != 5 {
			t.Errorf("Unexpected gauge vec value: %v", mf.Metric[0].Gauge)
		}
	}

	// Verify HistogramVec
	if mf, ok := familyMap["test_hist_vec"]; !ok {
		t.Error("Missing test_hist_vec")
	} else {
		if len(mf.Metric) != 1 || mf.Metric[0].Histogram.GetSampleCount() != 1 || mf.Metric[0].Histogram.GetSampleSum() != 1.5 {
			t.Errorf("Unexpected hist vec value: %v", mf.Metric[0].Histogram)
		}
	}

	// Verify Counter
	if mf, ok := familyMap["test_counter_single"]; !ok {
		t.Error("Missing test_counter_single")
	} else {
		if len(mf.Metric) != 1 || mf.Metric[0].Counter.GetValue() != 1 {
			t.Errorf("Unexpected counter value: %v", mf.Metric[0].Counter)
		}
	}

	// Verify Gauge
	if mf, ok := familyMap["test_gauge_single"]; !ok {
		t.Error("Missing test_gauge_single")
	} else {
		if len(mf.Metric) != 1 || mf.Metric[0].Gauge.GetValue() != 10 {
			t.Errorf("Unexpected gauge value: %v", mf.Metric[0].Gauge)
		}
	}

	// Verify Histogram
	if mf, ok := familyMap["test_hist_single"]; !ok {
		t.Error("Missing test_hist_single")
	} else {
		if len(mf.Metric) != 1 || mf.Metric[0].Histogram.GetSampleCount() != 1 || mf.Metric[0].Histogram.GetSampleSum() != 2.5 {
			t.Errorf("Unexpected histogram value: %v", mf.Metric[0].Histogram)
		}
	}
}

func TestStdMetricFactory_DuplicateRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	factory := NewStdMetricFactory(reg)

	opts := prometheus.CounterOpts{
		Name: "dup_counter",
		Help: "help",
	}

	// First registration
	c1, err := factory.NewCounterVec(opts, []string{"label1"})
	if err != nil {
		t.Fatalf("Failed to register first: %v", err)
	}

	// Second registration of same metric -> should return same collector (no error)
	c2, err := factory.NewCounterVec(opts, []string{"label1"})
	if err != nil {
		t.Fatalf("Expected no error on duplicate registration, got: %v", err)
	}

	c1.WithLabelValues("v1").Inc()
	c2.WithLabelValues("v1").Inc() // should increment same metric

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather: %v", err)
	}
	// Expected value is 2 because both c1 and c2 incremented it
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "dup_counter" {
			found = true
			if len(mf.Metric) != 1 {
				t.Fatalf("Expected 1 metric, got %d", len(mf.Metric))
			}
			if mf.Metric[0].Counter.GetValue() != 2 {
				t.Errorf("Expected counter value 2, got %f", mf.Metric[0].Counter.GetValue())
			}
		}
	}
	if !found {
		t.Error("Metric 'dup_counter' not found")
	}

	// Try to register with different type (Gauge) -> should fail
	_, err = factory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dup_counter",
		Help: "help",
	}, []string{"label1"})
	if err == nil {
		t.Error("Expected error when registering duplicate metric with different type, got nil")
	}
}
