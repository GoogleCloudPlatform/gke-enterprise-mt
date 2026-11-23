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
	"fmt"
	"sort"
	"sync"

	"k8s.io/klog/v2"
	"github.com/prometheus/client_golang/prometheus"
	dto_pb "github.com/prometheus/client_model/go"
)

// TenantGathererRegistry defines the interface for registering and
// unregistering Prometheus gatherers for individual tenants. It allows
// the multi-tenant metrics system to dynamically collect metrics from
// different tenants.
type TenantGathererRegistry interface {
	// Register registers a gatherer for a tenant identified by tenantUID.
	// Returns an error if a gatherer is already registered for this tenant.
	Register(tenantUID string, g prometheus.Gatherer) error
	// Unregister removes the gatherer associated with the tenantUID.
	Unregister(tenantUID string)
}

// MultiGatherer aggregates multiple prometheus Gatherers, typically one per tenant.
// It implements the prometheus.Gatherer interface to allow a single scraping endpoint
// to retrieve metrics from all registered tenants.
// It is thread-safe.
type MultiGatherer struct {
	mu               sync.RWMutex
	gatherers        map[string]prometheus.Gatherer
	processStartTime func() *dto_pb.MetricFamily
}

// NewMultiGatherer creates a new initialized MultiGatherer.
func NewMultiGatherer() *MultiGatherer {
	return &MultiGatherer{
		gatherers:        make(map[string]prometheus.Gatherer),
		processStartTime: cachedProcessStartTime,
	}
}

const processStartTimeMetric = "process_start_time_seconds"

var (
	processStartTimeOnce sync.Once
	processStartTime     *dto_pb.MetricFamily
)

// cachedProcessStartTime returns process_start_time_seconds from the process
// collector, gathered once since it never changes.
func cachedProcessStartTime() *dto_pb.MetricFamily {
	processStartTimeOnce.Do(func() {
		reg := prometheus.NewRegistry()
		if err := reg.Register(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{})); err != nil {
			klog.Warningf("Registering process collector for %s: %v", processStartTimeMetric, err)
			return
		}
		mfs, err := reg.Gather()
		if err != nil {
			klog.Warningf("Gathering %s: %v", processStartTimeMetric, err)
		}
		for _, mf := range mfs {
			if mf.GetName() == processStartTimeMetric {
				processStartTime = mf
				return
			}
		}
	})
	return processStartTime
}

// Register registers a gatherer for a tenant. Returns an error if the tenant
// is already registered.
func (m *MultiGatherer) Register(tenantUID string, g prometheus.Gatherer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.gatherers[tenantUID]; ok {
		return fmt.Errorf("tenant %q already registered", tenantUID)
	}
	m.gatherers[tenantUID] = g
	return nil
}

// Unregister removes the gatherer for a tenant. It is a no-op if the tenant
// is not registered.
func (m *MultiGatherer) Unregister(tenantUID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.gatherers, tenantUID)
}

// Gather implements prometheus.Gatherer. It gathers metrics from all registered
// tenant gatherers, merges them, and adds process_start_time_seconds.
// It uses a Copy-on-Read pattern to minimize lock hold duration and sorts
// gatherers by tenantUID to ensure deterministic metric order.
func (m *MultiGatherer) Gather() ([]*dto_pb.MetricFamily, error) {
	m.mu.RLock()
	tenantUIDs := make([]string, 0, len(m.gatherers))
	for uid := range m.gatherers {
		tenantUIDs = append(tenantUIDs, uid)
	}
	sort.Strings(tenantUIDs)

	var gatherers prometheus.Gatherers
	for _, uid := range tenantUIDs {
		gatherers = append(gatherers, m.gatherers[uid])
	}
	m.mu.RUnlock()

	mfs, err := gatherers.Gather()
	return m.addProcessStartTime(mfs), err
}

func (m *MultiGatherer) addProcessStartTime(mfs []*dto_pb.MetricFamily) []*dto_pb.MetricFamily {
	if len(mfs) == 0 || m.processStartTime == nil {
		return mfs
	}
	for _, mf := range mfs {
		if mf.GetName() == processStartTimeMetric {
			return mfs
		}
	}
	pst := m.processStartTime()
	if pst == nil {
		return mfs
	}
	mfs = append(mfs, pst)
	sort.Slice(mfs, func(i, j int) bool { return mfs[i].GetName() < mfs[j].GetName() })
	return mfs
}

// DefaultMultiGatherer is the default TenantGathererRegistry implementation
// shared across the application.
var DefaultMultiGatherer TenantGathererRegistry = NewMultiGatherer()
