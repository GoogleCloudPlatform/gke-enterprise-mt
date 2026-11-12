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
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto_pb "github.com/prometheus/client_model/go"
)

func TestMultiGatherer(t *testing.T) {
	mg := NewMultiGatherer()

	regA := prometheus.NewRegistry()
	vecA := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "m1", Help: "h1"}, []string{"tenant_uid", "label1"})
	regA.MustRegister(vecA)
	vecA.WithLabelValues("tenant-A", "val1").Inc()

	regB := prometheus.NewRegistry()
	vecB := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "m1", Help: "h1"}, []string{"tenant_uid", "label1"})
	regB.MustRegister(vecB)
	vecB.WithLabelValues("tenant-B", "val1").Add(2)

	if err := mg.Register("tenant-A", regA); err != nil {
		t.Fatalf("Failed to register tenant-A: %v", err)
	}
	if err := mg.Register("tenant-B", regB); err != nil {
		t.Fatalf("Failed to register tenant-B: %v", err)
	}

	// Try to register duplicate
	if err := mg.Register("tenant-A", regA); err == nil {
		t.Error("Expected error when registering duplicate tenant-A")
	}

	mfs, err := mg.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	mfs = withoutProcessStartTime(mfs)

	if len(mfs) != 1 {
		t.Fatalf("Expected 1 metric family, got %d", len(mfs))
	}

	mf := mfs[0]
	if mf.GetName() != "m1" {
		t.Errorf("Expected metric name 'm1', got %q", mf.GetName())
	}

	if len(mf.Metric) != 2 {
		t.Fatalf("Expected 2 metrics, got %d", len(mf.Metric))
	}

	// Unregister tenant-A
	mg.Unregister("tenant-A")

	mfs, err = mg.Gather()
	if err != nil {
		t.Fatalf("Gather failed after unregister: %v", err)
	}
	mfs = withoutProcessStartTime(mfs)

	if len(mfs) != 1 {
		t.Fatalf("Expected 1 metric family, got %d", len(mfs))
	}

	mf = mfs[0]
	if len(mf.Metric) != 1 {
		t.Fatalf("Expected 1 metric after unregister, got %d", len(mf.Metric))
	}

	labelsMap := make(map[string]string)
	for _, l := range mf.Metric[0].Label {
		labelsMap[l.GetName()] = l.GetValue()
	}
	if labelsMap["tenant_uid"] != "tenant-B" {
		t.Errorf("Expected remaining metric to be for tenant-B, got %q", labelsMap["tenant_uid"])
	}
}

func TestMultiGatherer_Empty(t *testing.T) {
	mg := NewMultiGatherer()
	mfs, err := mg.Gather()
	if err != nil {
		t.Fatalf("Gather failed for empty MultiGatherer: %v", err)
	}
	if len(mfs) != 0 {
		t.Errorf("Expected 0 metric families, got %d", len(mfs))
	}
}

type errorGatherer struct{}

func (e errorGatherer) Gather() ([]*dto_pb.MetricFamily, error) {
	return nil, errors.New("mock gather error")
}

func TestMultiGatherer_Error(t *testing.T) {
	mg := NewMultiGatherer()
	if err := mg.Register("tenant-A", errorGatherer{}); err != nil {
		t.Fatalf("Failed to register: %v", err)
	}

	_, err := mg.Gather()
	if err == nil {
		t.Error("Expected error from Gather, got nil")
	} else if !strings.Contains(err.Error(), "mock gather error") {
		t.Errorf("Expected error to contain 'mock gather error', got %q", err.Error())
	}
}

func TestMultiGatherer_Ordering(t *testing.T) {
	mg := NewMultiGatherer()

	// Register in non-alphabetical order
	regB := prometheus.NewRegistry()
	vecB := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "m1", Help: "h1"}, []string{"tenant_uid"})
	regB.MustRegister(vecB)
	vecB.WithLabelValues("tenant-B").Inc()

	regA := prometheus.NewRegistry()
	vecA := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "m1", Help: "h1"}, []string{"tenant_uid"})
	regA.MustRegister(vecA)
	vecA.WithLabelValues("tenant-A").Inc()

	if err := mg.Register("tenant-B", regB); err != nil {
		t.Fatalf("Failed to register tenant-B: %v", err)
	}
	if err := mg.Register("tenant-A", regA); err != nil {
		t.Fatalf("Failed to register tenant-A: %v", err)
	}

	mfs, err := mg.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	mfs = withoutProcessStartTime(mfs)

	if len(mfs) != 1 {
		t.Fatalf("Expected 1 metric family, got %d", len(mfs))
	}

	mf := mfs[0]
	if len(mf.Metric) != 2 {
		t.Fatalf("Expected 2 metrics, got %d", len(mf.Metric))
	}

	getTenantUID := func(m *dto_pb.Metric) string {
		for _, l := range m.Label {
			if l.GetName() == "tenant_uid" {
				return l.GetValue()
			}
		}
		return ""
	}

	if getTenantUID(mf.Metric[0]) != "tenant-A" {
		t.Errorf("Expected first metric to be tenant-A, got %q", getTenantUID(mf.Metric[0]))
	}
	if getTenantUID(mf.Metric[1]) != "tenant-B" {
		t.Errorf("Expected second metric to be tenant-B, got %q", getTenantUID(mf.Metric[1]))
	}
}

func withoutProcessStartTime(mfs []*dto_pb.MetricFamily) []*dto_pb.MetricFamily {
	var out []*dto_pb.MetricFamily
	for _, mf := range mfs {
		if mf.GetName() != processStartTimeMetric {
			out = append(out, mf)
		}
	}
	return out
}

func newTenantRegistry(t *testing.T, tenantUID string) *prometheus.Registry {
	t.Helper()
	reg := prometheus.NewRegistry()
	vec := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "m1", Help: "h1"}, []string{"tenant_uid"})
	reg.MustRegister(vec)
	vec.WithLabelValues(tenantUID).Inc()
	return reg
}

func TestMultiGatherer_AddsProcessStartTime(t *testing.T) {
	mg := NewMultiGatherer()
	if err := mg.Register("tenant-A", newTenantRegistry(t, "tenant-A")); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	mfs, err := mg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}
	var names []string
	for _, mf := range mfs {
		names = append(names, mf.GetName())
	}
	if want := []string{"m1", processStartTimeMetric}; !slices.Equal(names, want) {
		t.Fatalf("Gather() families = %v, want %v", names, want)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	pmfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("process collector Gather() failed: %v", err)
	}
	var want float64
	for _, mf := range pmfs {
		if mf.GetName() == processStartTimeMetric {
			want = mf.GetMetric()[0].GetGauge().GetValue()
		}
	}
	if got := mfs[1].GetMetric()[0].GetGauge().GetValue(); got != want || got == 0 {
		t.Errorf("%s = %v, want %v (non-zero, same as the process collector)", processStartTimeMetric, got, want)
	}
}

func TestMultiGatherer_KeepsTenantProcessStartTime(t *testing.T) {
	mg := NewMultiGatherer()
	reg := newTenantRegistry(t, "tenant-A")
	pst := prometheus.NewGauge(prometheus.GaugeOpts{Name: processStartTimeMetric, Help: "Start time."})
	reg.MustRegister(pst)
	pst.Set(42)
	if err := mg.Register("tenant-A", reg); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	mfs, err := mg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}
	var got []float64
	for _, mf := range mfs {
		if mf.GetName() == processStartTimeMetric {
			for _, m := range mf.GetMetric() {
				got = append(got, m.GetGauge().GetValue())
			}
		}
	}
	if !slices.Equal(got, []float64{42}) {
		t.Errorf("%s values = %v, want [42] (tenant value kept, not duplicated)", processStartTimeMetric, got)
	}
}

func TestMultiGatherer_NoProcessStartTimeWithoutTenantMetrics(t *testing.T) {
	mg := NewMultiGatherer()
	if err := mg.Register("tenant-A", prometheus.NewRegistry()); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}
	mfs, err := mg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}
	if len(mfs) != 0 {
		t.Errorf("Gather() returned %d families, want 0 when tenants have no metrics", len(mfs))
	}
}

func TestMultiGatherer_ProcessStartTimeUnavailable(t *testing.T) {
	mg := NewMultiGatherer()
	mg.processStartTime = func() *dto_pb.MetricFamily { return nil }
	if err := mg.Register("tenant-A", newTenantRegistry(t, "tenant-A")); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}
	mfs, err := mg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}
	if len(mfs) != 1 || mfs[0].GetName() != "m1" {
		t.Errorf("Gather() = %v, want only m1 when the start time is unavailable", mfs)
	}
}
