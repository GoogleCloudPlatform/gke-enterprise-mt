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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)


type AggregationStrategy int

const (
	StrategySum AggregationStrategy = iota
	StrategyMax
	StrategyMin
)

// GlobalMetricsTracker coordinates registration of global (shared) metrics
// to prevent duplicate registration panics in Prometheus. It also aggregates
// Gauge metric values across multiple tenants. For Gauge metrics, instead of
// exposing tenant-specific gauges, GlobalMetricsTracker maintains the individual
// tenant values and sets the global gauge to the sum of all tenant values.
//
// It is thread-safe.
type GlobalMetricsTracker struct {
	mu              sync.Mutex
	collectors      map[string]prometheus.Collector
	// gaugeValues maps: metric_name -> serialized_labels -> tenant_uid -> value
	gaugeValues     map[string]map[string]map[string]float64
	metricLabels    map[string][]string
	gaugeStrategies map[string]AggregationStrategy
}

// NewGlobalMetricsTracker creates and returns a new GlobalMetricsTracker.
func NewGlobalMetricsTracker() *GlobalMetricsTracker {
	return &GlobalMetricsTracker{
		collectors:      make(map[string]prometheus.Collector),
		gaugeValues:     make(map[string]map[string]map[string]float64),
		metricLabels:    make(map[string][]string),
		gaugeStrategies: make(map[string]AggregationStrategy),
	}
}


// SetAggregationStrategy sets the aggregation strategy for a specific metric name.
// It is thread-safe.
func (t *GlobalMetricsTracker) SetAggregationStrategy(name string, strategy AggregationStrategy) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.gaugeStrategies == nil {
		t.gaugeStrategies = make(map[string]AggregationStrategy)
	}
	t.gaugeStrategies[name] = strategy
}

// serializeLabels converts a slice of label values into a unique string key.
func serializeLabels(lvs []string) string {
	return strconv.Itoa(len(lvs)) + "\x00" + strings.Join(lvs, "\x00")
}

// deserializeLabels converts a serialized label key back into a slice of label values.
func deserializeLabels(key string) []string {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) < 2 {
		return []string{}
	}
	count, err := strconv.Atoi(parts[0])
	if err != nil || count <= 0 {
		return []string{}
	}

	return strings.SplitN(parts[1], "\x00", count)
}

// SetGaugeValue sets the gauge value for a specific tenant and label values combination,
// and updates the global gauge with the aggregated sum.
func (t *GlobalMetricsTracker) SetGaugeValue(name string, lvs []string, tenantUID string, val float64) {
	t.mu.Lock()
	t.setGaugeValueLocked(name, lvs, tenantUID, val)
	t.mu.Unlock()
}

func (t *GlobalMetricsTracker) setGaugeValueLocked(name string, lvs []string, tenantUID string, val float64) {
	if t.gaugeValues == nil {
		t.gaugeValues = make(map[string]map[string]map[string]float64)
	}
	if t.gaugeValues[name] == nil {
		t.gaugeValues[name] = make(map[string]map[string]float64)
	}
	lvsKey := serializeLabels(lvs)
	if t.gaugeValues[name][lvsKey] == nil {
		t.gaugeValues[name][lvsKey] = make(map[string]float64)
	}

	t.gaugeValues[name][lvsKey][tenantUID] = val
	t.updateGlobalGaugeLocked(name, lvs, lvsKey)
}

// AddGaugeValue adds a delta to the gauge value for a specific tenant and label
// values combination, and updates the global gauge with the aggregated sum.
func (t *GlobalMetricsTracker) AddGaugeValue(name string, lvs []string, tenantUID string, delta float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.gaugeValues == nil {
		t.gaugeValues = make(map[string]map[string]map[string]float64)
	}
	if t.gaugeValues[name] == nil {
		t.gaugeValues[name] = make(map[string]map[string]float64)
	}
	lvsKey := serializeLabels(lvs)
	if t.gaugeValues[name][lvsKey] == nil {
		t.gaugeValues[name][lvsKey] = make(map[string]float64)
	}

	t.gaugeValues[name][lvsKey][tenantUID] += delta
	t.updateGlobalGaugeLocked(name, lvs, lvsKey)
}

func (t *GlobalMetricsTracker) deleteGaugeLabelValuesLocked(name string, lvs []string, tenantUID string) bool {
	if t.gaugeValues == nil || t.gaugeValues[name] == nil {
		return false
	}
	lvsKey := serializeLabels(lvs)
	tenantMap, ok := t.gaugeValues[name][lvsKey]
	if !ok {
		return false
	}
	if _, ok := tenantMap[tenantUID]; !ok {
		return false
	}
	delete(tenantMap, tenantUID)
	t.updateGlobalGaugeLocked(name, lvs, lvsKey)
	return true
}

// DeleteGaugeLabelValues removes the gauge value for a specific tenant and label
// values combination, and updates the global gauge. Returns true if the value existed.
func (t *GlobalMetricsTracker) DeleteGaugeLabelValues(name string, lvs []string, tenantUID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.deleteGaugeLabelValuesLocked(name, lvs, tenantUID)
}

// ResetGaugeVec removes all gauge values for a specific metric name and tenant,
// updating the global gauge accordingly.
func (t *GlobalMetricsTracker) ResetGaugeVec(name string, tenantUID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.gaugeValues == nil || t.gaugeValues[name] == nil {
		return
	}

	var keysToDelete []string
	for lvsKey, tenantMap := range t.gaugeValues[name] {
		if _, ok := tenantMap[tenantUID]; ok {
			keysToDelete = append(keysToDelete, lvsKey)
		}
	}
	for _, lvsKey := range keysToDelete {
		lvs := deserializeLabels(lvsKey)
		t.deleteGaugeLabelValuesLocked(name, lvs, tenantUID)
	}
}

// ResetTenant removes all gauge values across all metrics for a specific tenant.
// This is typically called when a tenant is deleted or cleaned up.
func (t *GlobalMetricsTracker) ResetTenant(tenantUID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.gaugeValues == nil {
		return
	}

	type metricLvs struct {
		name   string
		lvsKey string
	}
	var targets []metricLvs

	for name, labelMap := range t.gaugeValues {
		for lvsKey, tenantMap := range labelMap {
			if _, ok := tenantMap[tenantUID]; ok {
				targets = append(targets, metricLvs{name: name, lvsKey: lvsKey})
			}
		}
	}

	for _, target := range targets {
		lvs := deserializeLabels(target.lvsKey)
		t.deleteGaugeLabelValuesLocked(target.name, lvs, tenantUID)
	}
}

func aggregateValues(values map[string]float64, strategy AggregationStrategy) float64 {
	if len(values) == 0 {
		return 0
	}
	var result float64
	first := true
	for _, val := range values {
		if first {
			result = val
			first = false
			continue
		}
		switch strategy {
		case StrategyMax:
			if val > result {
				result = val
			}
		case StrategyMin:
			if val < result {
				result = val
			}
		default: // StrategySum
			result += val
		}
	}
	return result
}

// updateGlobalGaugeLocked updates the global aggregated gauge with the aggregated value
// (sum, max, or min) of all registered tenant gauge values. It handles both GaugeVec
// and scalar Gauge.
// The caller must hold the tracker lock.
func (t *GlobalMetricsTracker) updateGlobalGaugeLocked(name string, lvs []string, lvsKey string) {
	// We compute the sum on every update by iterating over all tenant values
	// rather than maintaining a running sum. This prevents floating-point
	// precision drift over time and cleanly supports Min/Max aggregation strategies.
	// Since the number of tenants per cluster is typically small, the performance
	// overhead of this iteration is negligible.
	c, ok := t.collectors[name]
	if !ok {
		return
	}
	strategy := StrategySum
	if t.gaugeStrategies != nil {
		strategy = t.gaugeStrategies[name]
	}

	if gv, ok := c.(*prometheus.GaugeVec); ok {
		if len(t.gaugeValues[name][lvsKey]) == 0 {
			gv.DeleteLabelValues(lvs...)
			delete(t.gaugeValues[name], lvsKey)
			if len(t.gaugeValues[name]) == 0 {
				delete(t.gaugeValues, name)
			}
			return
		}

		val := aggregateValues(t.gaugeValues[name][lvsKey], strategy)
		gv.WithLabelValues(lvs...).Set(val)
	} else if g, ok := c.(prometheus.Gauge); ok {
		if len(t.gaugeValues[name][lvsKey]) == 0 {
			g.Set(0)
			delete(t.gaugeValues[name], lvsKey)
			if len(t.gaugeValues[name]) == 0 {
				delete(t.gaugeValues, name)
			}
			return
		}
		val := aggregateValues(t.gaugeValues[name][lvsKey], strategy)
		g.Set(val)
	}
}

// validateLabelsLocked validates that the provided labelNames match the labelNames
// of the metric if it is already registered, preventing label mismatch conflicts.
// If the metric is not registered, it records the labelNames.
// The caller must hold the tracker lock.
func (t *GlobalMetricsTracker) validateLabelsLocked(name string, labelNames []string) error {
	if t.collectors == nil {
		t.collectors = make(map[string]prometheus.Collector)
	}
	if t.metricLabels == nil {
		t.metricLabels = make(map[string][]string)
	}

	if storedLabels, ok := t.metricLabels[name]; ok {
		if len(storedLabels) != len(labelNames) {
			return fmt.Errorf("metric %q already registered with different labels: expected %v, got %v", name, storedLabels, labelNames)
		}
		for i, l := range storedLabels {
			if l != labelNames[i] {
				return fmt.Errorf("metric %q already registered with different labels: expected %v, got %v", name, storedLabels, labelNames)
			}
		}
	} else {
		// Create a copy of the slice to avoid external mutation side-effects.
		copiedLabels := make([]string, len(labelNames))
		copy(copiedLabels, labelNames)
		t.metricLabels[name] = copiedLabels
	}
	return nil
}

// getOrCreateCounterVec retrieves the existing global CounterVec matching the name
// and labelNames, or registers a new one if not found.
func (t *GlobalMetricsTracker) getOrCreateCounterVec(opts prometheus.CounterOpts, labelNames []string, reg prometheus.Registerer) (*prometheus.CounterVec, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	if err := t.validateLabelsLocked(key, labelNames); err != nil {
		return nil, err
	}
	if c, ok := t.collectors[key]; ok {
		cv, ok := c.(*prometheus.CounterVec)
		if !ok {
			return nil, fmt.Errorf("metric %s already registered with different type", key)
		}
		return cv, nil
	}
	cv := prometheus.NewCounterVec(opts, labelNames)
	if err := reg.Register(cv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			cv, okAssert = are.ExistingCollector.(*prometheus.CounterVec)
			if !okAssert {
				return nil, fmt.Errorf("metric registered but not of type *prometheus.CounterVec: %w", err)
			}
		} else {
			return nil, err
		}
	}
	t.collectors[key] = cv
	return cv, nil
}

// getOrCreateGaugeVec retrieves the existing global GaugeVec matching the name
// and labelNames, or registers a new one if not found.
func (t *GlobalMetricsTracker) getOrCreateGaugeVec(opts prometheus.GaugeOpts, labelNames []string, reg prometheus.Registerer) (*prometheus.GaugeVec, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	if err := t.validateLabelsLocked(key, labelNames); err != nil {
		return nil, err
	}
	if c, ok := t.collectors[key]; ok {
		gv, ok := c.(*prometheus.GaugeVec)
		if !ok {
			return nil, fmt.Errorf("metric %s already registered with different type", key)
		}
		return gv, nil
	}
	gv := prometheus.NewGaugeVec(opts, labelNames)
	if err := reg.Register(gv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			gv, okAssert = are.ExistingCollector.(*prometheus.GaugeVec)
			if !okAssert {
				return nil, fmt.Errorf("metric registered but not of type *prometheus.GaugeVec: %w", err)
			}
		} else {
			return nil, err
		}
	}
	t.collectors[key] = gv
	return gv, nil
}

// getOrCreateHistogramVec retrieves the existing global HistogramVec matching the name
// and labelNames, or registers a new one if not found.
func (t *GlobalMetricsTracker) getOrCreateHistogramVec(opts prometheus.HistogramOpts, labelNames []string, reg prometheus.Registerer) (*prometheus.HistogramVec, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	if err := t.validateLabelsLocked(key, labelNames); err != nil {
		return nil, err
	}
	if c, ok := t.collectors[key]; ok {
		hv, ok := c.(*prometheus.HistogramVec)
		if !ok {
			return nil, fmt.Errorf("metric %s already registered with different type", key)
		}
		return hv, nil
	}
	hv := prometheus.NewHistogramVec(opts, labelNames)
	if err := reg.Register(hv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			hv, okAssert = are.ExistingCollector.(*prometheus.HistogramVec)
			if !okAssert {
				return nil, fmt.Errorf("metric registered but not of type *prometheus.HistogramVec: %w", err)
			}
		} else {
			return nil, err
		}
	}
	t.collectors[key] = hv
	return hv, nil
}

// getOrCreateCounter retrieves the existing global scalar Counter matching the name,
// or registers a new one if not found.
func (t *GlobalMetricsTracker) getOrCreateCounter(opts prometheus.CounterOpts, reg prometheus.Registerer) (prometheus.Counter, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	if err := t.validateLabelsLocked(key, []string{}); err != nil {
		return nil, err
	}
	if c, ok := t.collectors[key]; ok {
		cv, ok := c.(prometheus.Counter)
		if !ok {
			return nil, fmt.Errorf("metric %s already registered with different type", key)
		}
		return cv, nil
	}
	cv := prometheus.NewCounter(opts)
	if err := reg.Register(cv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			cv, okAssert = are.ExistingCollector.(prometheus.Counter)
			if !okAssert {
				return nil, fmt.Errorf("metric registered but not of type prometheus.Counter: %w", err)
			}
		} else {
			return nil, err
		}
	}
	t.collectors[key] = cv
	return cv, nil
}

// getOrCreateGauge retrieves the existing global scalar Gauge matching the name,
// or registers a new one if not found.
func (t *GlobalMetricsTracker) getOrCreateGauge(opts prometheus.GaugeOpts, reg prometheus.Registerer) (prometheus.Gauge, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	if err := t.validateLabelsLocked(key, []string{}); err != nil {
		return nil, err
	}
	if c, ok := t.collectors[key]; ok {
		gv, ok := c.(prometheus.Gauge)
		if !ok {
			return nil, fmt.Errorf("metric %s already registered with different type", key)
		}
		return gv, nil
	}
	gv := prometheus.NewGauge(opts)
	if err := reg.Register(gv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			gv, okAssert = are.ExistingCollector.(prometheus.Gauge)
			if !okAssert {
				return nil, fmt.Errorf("metric registered but not of type prometheus.Gauge: %w", err)
			}
		} else {
			return nil, err
		}
	}
	t.collectors[key] = gv
	return gv, nil
}

// getOrCreateHistogram retrieves the existing global scalar Histogram matching the name,
// or registers a new one if not found.
func (t *GlobalMetricsTracker) getOrCreateHistogram(opts prometheus.HistogramOpts, reg prometheus.Registerer) (prometheus.Histogram, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	if err := t.validateLabelsLocked(key, []string{}); err != nil {
		return nil, err
	}
	if c, ok := t.collectors[key]; ok {
		hv, ok := c.(prometheus.Histogram)
		if !ok {
			return nil, fmt.Errorf("metric %s already registered with different type", key)
		}
		return hv, nil
	}
	hv := prometheus.NewHistogram(opts)
	if err := reg.Register(hv); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			hv, okAssert = are.ExistingCollector.(prometheus.Histogram)
			if !okAssert {
				return nil, fmt.Errorf("metric registered but not of type prometheus.Histogram: %w", err)
			}
		} else {
			return nil, err
		}
	}
	t.collectors[key] = hv
	return hv, nil
}


// mtMetricFactory is a MetricFactory implementation for multi-tenant mode.
// It supports dual-emission: metrics are registered to both a tenant-local
// registry (for local scraping) and a global registry (for aggregated global view).
// For Gauges, it coordinates with a GlobalMetricsTracker to aggregate values.
type mtMetricFactory struct {
	tenantUID     string
	globalReg     prometheus.Registerer
	localRegistry *prometheus.Registry
	tracker       *GlobalMetricsTracker
}

// NewMTMetricFactory creates a new MetricFactory for a specific tenant.
// It requires the tenant's UID, the global registry to register global metrics,
// and the GlobalMetricsTracker for aggregating gauge values.
func NewMTMetricFactory(tenantUID string, globalReg prometheus.Registerer, tracker *GlobalMetricsTracker) MetricFactory {
	return &mtMetricFactory{
		tenantUID:     tenantUID,
		globalReg:     globalReg,
		localRegistry: prometheus.NewRegistry(),
		tracker:       tracker,
	}
}

// Registry returns the tenant-local registry. This registry contains only
// metrics created by this factory, and all of them will have a "tenant_uid"
// label appended.
func (f *mtMetricFactory) Registry() *prometheus.Registry {
	return f.localRegistry
}

// Cleanup cleans up the tenant's metrics in the GlobalMetricsTracker.
// It should be called when the tenant is being decommissioned or the factory is discarded.
func (f *mtMetricFactory) Cleanup() {
	if f.tracker != nil {
		f.tracker.ResetTenant(f.tenantUID)
	}
}

// Wrappers that implement dual emission

// mtCounter wraps a local prometheus.Counter and a global prometheus.Counter
// to implement dual-emission of increments.
type mtCounter struct {
	prometheus.Counter
	global prometheus.Counter
}

// Inc increments both local and global counters by 1.
func (c mtCounter) Inc() {
	if c.Counter != nil {
		c.Counter.Inc()
	}
	if c.global != nil {
		c.global.Inc()
	}
}

// Add adds the given value to both local and global counters.
func (c mtCounter) Add(v float64) {
	if c.Counter != nil {
		c.Counter.Add(v)
	}
	if c.global != nil {
		c.global.Add(v)
	}
}

// mtGauge wraps a local tenant-specific prometheus.Gauge and propagates
// updates to a shared GlobalMetricsTracker instead of a direct global Gauge.
type mtGauge struct {
	prometheus.Gauge
	tracker    *GlobalMetricsTracker
	metricName string
	lvs        []string
	tenantUID  string
}

// Set sets the gauge value for the tenant locally and updates the aggregated
// value in the tracker.
func (g mtGauge) Set(v float64) {
	if g.Gauge != nil {
		g.Gauge.Set(v)
	}
	if g.tracker != nil {
		g.tracker.SetGaugeValue(g.metricName, g.lvs, g.tenantUID, v)
	}
}

// Inc increments the gauge value by 1 locally and updates the tracker.
func (g mtGauge) Inc() {
	if g.Gauge != nil {
		g.Gauge.Inc()
	}
	if g.tracker != nil {
		g.tracker.AddGaugeValue(g.metricName, g.lvs, g.tenantUID, 1)
	}
}

// Dec decrements the gauge value by 1 locally and updates the tracker.
func (g mtGauge) Dec() {
	if g.Gauge != nil {
		g.Gauge.Dec()
	}
	if g.tracker != nil {
		g.tracker.AddGaugeValue(g.metricName, g.lvs, g.tenantUID, -1)
	}
}

// Add adds the given value to the gauge locally and updates the tracker.
func (g mtGauge) Add(v float64) {
	if g.Gauge != nil {
		g.Gauge.Add(v)
	}
	if g.tracker != nil {
		g.tracker.AddGaugeValue(g.metricName, g.lvs, g.tenantUID, v)
	}
}

// Sub subtracts the given value from the gauge locally and updates the tracker.
func (g mtGauge) Sub(v float64) {
	if g.Gauge != nil {
		g.Gauge.Sub(v)
	}
	if g.tracker != nil {
		g.tracker.AddGaugeValue(g.metricName, g.lvs, g.tenantUID, -v)
	}
}

// SetToCurrentTime sets the gauge to the current Unix time in seconds,
// locally and in the tracker.
func (g mtGauge) SetToCurrentTime() {
	val := float64(time.Now().UnixNano()) / 1e9
	if g.Gauge != nil {
		g.Gauge.Set(val)
	}
	if g.tracker != nil {
		g.tracker.SetGaugeValue(g.metricName, g.lvs, g.tenantUID, val)
	}
}

// mtObserver wraps a local and global observer (typically used in histogram vectors)
// to implement dual-emission of observations.
type mtObserver struct {
	prometheus.Observer
	global prometheus.Observer
}

// Observe records the observation in both local and global observers.
func (o mtObserver) Observe(v float64) {
	if o.Observer != nil {
		o.Observer.Observe(v)
	}
	if o.global != nil {
		o.global.Observe(v)
	}
}

// mtHistogram wraps a local prometheus.Histogram and a global prometheus.Observer
// (from the global histogram vec) to implement dual-emission of observations
// for scalar histograms.
type mtHistogram struct {
	prometheus.Histogram
	global prometheus.Observer
}

// Observe records the observation in both local histogram and global observer.
func (h mtHistogram) Observe(v float64) {
	if h.Histogram != nil {
		h.Histogram.Observe(v)
	}
	if h.global != nil {
		h.global.Observe(v)
	}
}

// Vectors

// mtCounterVec is a CounterVec implementation that supports dual-emission.
// It automatically appends "tenant_uid" to the local metric labels.
type mtCounterVec struct {
	tenantUID string
	local     *prometheus.CounterVec
	global    *prometheus.CounterVec
}

// WithLabelValues returns a Counter wrapper for the given label values.
// The local Counter will have the tenantUID appended as a label, while the
// global Counter will only use the provided label values.
func (v *mtCounterVec) WithLabelValues(lvs ...string) prometheus.Counter {
	tenantLvs := append(append([]string(nil), lvs...), v.tenantUID)
	var local prometheus.Counter
	if v.local != nil {
		local = v.local.WithLabelValues(tenantLvs...)
	}
	var global prometheus.Counter
	if v.global != nil {
		global = v.global.WithLabelValues(lvs...)
	}
	return mtCounter{Counter: local, global: global}
}

// Reset resets the local CounterVec. It does not affect the global CounterVec.
func (v *mtCounterVec) Reset() {
	if v.local != nil {
		v.local.Reset()
	}
}

// mtGaugeVec is a GaugeVec implementation that supports dual-emission.
// It automatically appends "tenant_uid" to the local metric labels, and
// aggregates global values using a GlobalMetricsTracker.
type mtGaugeVec struct {
	tenantUID  string
	metricName string
	local      *prometheus.GaugeVec
	tracker    *GlobalMetricsTracker
}

// WithLabelValues returns a Gauge wrapper for the given label values.
// The local Gauge will have the tenantUID appended as a label.
// Updates to the returned Gauge will propagate to the local Gauge and the tracker.
func (v *mtGaugeVec) WithLabelValues(lvs ...string) prometheus.Gauge {
	tenantLvs := append(append([]string(nil), lvs...), v.tenantUID)
	var local prometheus.Gauge
	if v.local != nil {
		local = v.local.WithLabelValues(tenantLvs...)
	}
	return mtGauge{
		Gauge:      local,
		tracker:    v.tracker,
		metricName: v.metricName,
		lvs:        lvs,
		tenantUID:  v.tenantUID,
	}
}

// DeleteLabelValues deletes the metric for the given label values.
// It deletes from the local GaugeVec and also removes the tenant's entry
// from the tracker for these label values.
func (v *mtGaugeVec) DeleteLabelValues(lvs ...string) bool {
	tenantLvs := append(append([]string(nil), lvs...), v.tenantUID)
	localDeleted := false
	if v.local != nil {
		localDeleted = v.local.DeleteLabelValues(tenantLvs...)
	}
	trackerDeleted := false
	if v.tracker != nil {
		trackerDeleted = v.tracker.DeleteGaugeLabelValues(v.metricName, lvs, v.tenantUID)
	}
	return localDeleted || trackerDeleted
}

// Reset resets the local GaugeVec and removes all values for this tenant and
// metric from the tracker.
func (v *mtGaugeVec) Reset() {
	if v.local != nil {
		v.local.Reset()
	}
	if v.tracker != nil {
		v.tracker.ResetGaugeVec(v.metricName, v.tenantUID)
	}
}

// mtObserverVec is an ObserverVec implementation that supports dual-emission
// (typically wrapping histograms).
// It automatically appends "tenant_uid" to the local metric labels.
type mtObserverVec struct {
	tenantUID string
	local     *prometheus.HistogramVec
	global    *prometheus.HistogramVec
}

// WithLabelValues returns an Observer wrapper for the given label values.
// The local Observer will have the tenantUID appended as a label.
func (v *mtObserverVec) WithLabelValues(lvs ...string) prometheus.Observer {
	tenantLvs := append(append([]string(nil), lvs...), v.tenantUID)
	var local prometheus.Observer
	if v.local != nil {
		local = v.local.WithLabelValues(tenantLvs...)
	}
	var global prometheus.Observer
	if v.global != nil {
		global = v.global.WithLabelValues(lvs...)
	}
	return mtObserver{Observer: local, global: global}
}

// Reset resets the local HistogramVec. It does not affect the global HistogramVec.
func (v *mtObserverVec) Reset() {
	if v.local != nil {
		v.local.Reset()
	}
}

// Factory methods

// NewCounterVec creates a new CounterVec.
// The local CounterVec will automatically have "tenant_uid" added to its labels.
// The global CounterVec will use the original labelNames and is registered
// to the global registry via the tracker.
func (f *mtMetricFactory) NewCounterVec(opts prometheus.CounterOpts, labelNames []string) (CounterVec, error) {
	mtLabelNames := append(append([]string(nil), labelNames...), "tenant_uid")
	globalVec, err := f.tracker.getOrCreateCounterVec(opts, labelNames, f.globalReg)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create global counter vec: %w", err)
	}

	localOpts := opts
	localVec := prometheus.NewCounterVec(localOpts, mtLabelNames)

	if err := f.localRegistry.Register(localVec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			localVec, okAssert = are.ExistingCollector.(*prometheus.CounterVec)
			if !okAssert {
				return nil, fmt.Errorf("local counter vec %q already registered with different type", opts.Name)
			}
		} else {
			return nil, fmt.Errorf("failed to register local counter vec: %w", err)
		}
	}

	return &mtCounterVec{
		tenantUID: f.tenantUID,
		local:     localVec,
		global:    globalVec,
	}, nil
}

// NewGaugeVec creates a new GaugeVec.
// The local GaugeVec will automatically have "tenant_uid" added to its labels.
// The global GaugeVec is managed by the tracker and registered to the global registry.
func (f *mtMetricFactory) NewGaugeVec(opts prometheus.GaugeOpts, labelNames []string) (GaugeVec, error) {
	mtLabelNames := append(append([]string(nil), labelNames...), "tenant_uid")
	_, err := f.tracker.getOrCreateGaugeVec(opts, labelNames, f.globalReg)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create global gauge vec: %w", err)
	}

	localOpts := opts
	localVec := prometheus.NewGaugeVec(localOpts, mtLabelNames)

	if err := f.localRegistry.Register(localVec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			localVec, okAssert = are.ExistingCollector.(*prometheus.GaugeVec)
			if !okAssert {
				return nil, fmt.Errorf("local gauge vec %q already registered with different type", opts.Name)
			}
		} else {
			return nil, fmt.Errorf("failed to register local gauge vec: %w", err)
		}
	}

	return &mtGaugeVec{
		tenantUID:  f.tenantUID,
		metricName: prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		local:      localVec,
		tracker:    f.tracker,
	}, nil
}

// NewHistogramVec creates a new HistogramVec (returned as ObserverVec).
// The local HistogramVec will automatically have "tenant_uid" added to its labels.
// The global HistogramVec is registered to the global registry via the tracker.
func (f *mtMetricFactory) NewHistogramVec(opts prometheus.HistogramOpts, labelNames []string) (ObserverVec, error) {
	mtLabelNames := append(append([]string(nil), labelNames...), "tenant_uid")
	globalVec, err := f.tracker.getOrCreateHistogramVec(opts, labelNames, f.globalReg)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create global histogram vec: %w", err)
	}

	localOpts := opts
	localVec := prometheus.NewHistogramVec(localOpts, mtLabelNames)

	if err := f.localRegistry.Register(localVec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			localVec, okAssert = are.ExistingCollector.(*prometheus.HistogramVec)
			if !okAssert {
				return nil, fmt.Errorf("local histogram vec %q already registered with different type", opts.Name)
			}
		} else {
			return nil, fmt.Errorf("failed to register local histogram vec: %w", err)
		}
	}

	return &mtObserverVec{
		tenantUID: f.tenantUID,
		local:     localVec,
		global:    globalVec,
	}, nil
}

// NewCounter creates a new Counter.
// Locally it is implemented as a CounterVec with a single label "tenant_uid"
// to allow partitioning by tenant in the local registry.
// Globally it is registered as a standard Counter.
func (f *mtMetricFactory) NewCounter(opts prometheus.CounterOpts) (prometheus.Counter, error) {
	globalCounter, err := f.tracker.getOrCreateCounter(opts, f.globalReg)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create global counter: %w", err)
	}

	mtLabelNames := []string{"tenant_uid"}
	localVec := prometheus.NewCounterVec(opts, mtLabelNames)
	if err := f.localRegistry.Register(localVec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			localVec, okAssert = are.ExistingCollector.(*prometheus.CounterVec)
			if !okAssert {
				return nil, fmt.Errorf("local metric registered but not of type *prometheus.CounterVec: %w", err)
			}
		} else {
			return nil, err
		}
	}

	return mtCounter{
		Counter: localVec.WithLabelValues(f.tenantUID),
		global:  globalCounter,
	}, nil
}

// NewGauge creates a new Gauge.
// Locally it is implemented as a GaugeVec with a single label "tenant_uid".
// Globally it is aggregated by the tracker.
func (f *mtMetricFactory) NewGauge(opts prometheus.GaugeOpts) (prometheus.Gauge, error) {
	_, err := f.tracker.getOrCreateGauge(opts, f.globalReg)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create global gauge: %w", err)
	}

	mtLabelNames := []string{"tenant_uid"}
	localVec := prometheus.NewGaugeVec(opts, mtLabelNames)
	if err := f.localRegistry.Register(localVec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			localVec, okAssert = are.ExistingCollector.(*prometheus.GaugeVec)
			if !okAssert {
				return nil, fmt.Errorf("local metric registered but not of type *prometheus.GaugeVec: %w", err)
			}
		} else {
			return nil, err
		}
	}

	metricName := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	return mtGauge{
		Gauge:      localVec.WithLabelValues(f.tenantUID),
		tracker:    f.tracker,
		metricName: metricName,
		lvs:        []string{},
		tenantUID:  f.tenantUID,
	}, nil
}

// NewHistogram creates a new Histogram.
// Locally it is implemented as a HistogramVec with a single label "tenant_uid".
// Globally it is registered as a standard Histogram.
func (f *mtMetricFactory) NewHistogram(opts prometheus.HistogramOpts) (prometheus.Histogram, error) {
	globalHist, err := f.tracker.getOrCreateHistogram(opts, f.globalReg)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create global histogram: %w", err)
	}

	mtLabelNames := []string{"tenant_uid"}
	localVec := prometheus.NewHistogramVec(opts, mtLabelNames)
	if err := f.localRegistry.Register(localVec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			var okAssert bool
			localVec, okAssert = are.ExistingCollector.(*prometheus.HistogramVec)
			if !okAssert {
				return nil, fmt.Errorf("local metric registered but not of type *prometheus.HistogramVec: %w", err)
			}
		} else {
			return nil, err
		}
	}

	localObs := localVec.WithLabelValues(f.tenantUID)
	localHist, ok := localObs.(prometheus.Histogram)
	if !ok {
		return nil, fmt.Errorf("failed to cast local observer to Histogram")
	}

	return mtHistogram{
		Histogram: localHist,
		global:    globalHist,
	}, nil
}
