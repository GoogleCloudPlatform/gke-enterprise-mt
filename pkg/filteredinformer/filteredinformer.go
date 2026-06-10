// Package filteredinformer implements informer with provider config filtering.
package filteredinformer

import (
	"sync/atomic"
	"time"

	"k8s.io/client-go/tools/cache"
)

// ProviderConfigFilteredInformer wraps a SharedIndexInformer to provide a filtered view.
type ProviderConfigFilteredInformer struct {
	cache.SharedIndexInformer
	filterKey    string
	filterValue  string
	allowMissing bool
	stopped      atomic.Bool
}

// NewFilteredInformer creates a new generic FilteredInformer (internally named ProviderConfigFilteredInformer for compatibility).
func NewFilteredInformer(informer cache.SharedIndexInformer, filterKey, filterValue string, allowMissing bool) cache.SharedIndexInformer {
	indexers := informer.GetIndexer().GetIndexers()
	if indexers != nil {
		if _, ok := indexers[filterKey]; !ok {
			informer.AddIndexers(cache.Indexers{filterKey: NewLabelIndexFunc(filterKey)})
		}
	}
	return &ProviderConfigFilteredInformer{
		SharedIndexInformer: informer,
		filterKey:           filterKey,
		filterValue:         filterValue,
		allowMissing:        allowMissing,
	}
}

// NewProviderConfigFilteredInformer creates a new ProviderConfigFilteredInformer (legacy constructor).
func NewProviderConfigFilteredInformer(informer cache.SharedIndexInformer, providerConfigName string) cache.SharedIndexInformer {
	return NewFilteredInformer(informer, providerConfigLabel, providerConfigName, false)
}

// AddEventHandler adds an event handler that only processes events matching the filter.
func (i *ProviderConfigFilteredInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	i.SharedIndexInformer.AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: i.filterFunc,
			Handler:    handler,
		},
	)
}

// AddEventHandlerWithResyncPeriod adds an event handler with resync period.
func (i *ProviderConfigFilteredInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) {
	i.SharedIndexInformer.AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			FilterFunc: i.filterFunc,
			Handler:    handler,
		},
		resyncPeriod,
	)
}

// filterFunc filters objects based on the configured key and value.
func (i *ProviderConfigFilteredInformer) filterFunc(obj any) bool {
	if i.stopped.Load() {
		return false
	}
	return isObjectMatchingValue(obj, i.filterKey, i.filterValue, i.allowMissing)
}

// GetStore returns a Store that only stores objects matching the filter.
func (i *ProviderConfigFilteredInformer) GetStore() cache.Store {
	return &providerConfigFilteredCache{
		Indexer:      i.SharedIndexInformer.GetIndexer(),
		filterKey:    i.filterKey,
		filterValue:  i.filterValue,
		allowMissing: i.allowMissing,
	}
}

// GetIndexer returns an Indexer that only indexes objects matching the filter.
func (i *ProviderConfigFilteredInformer) GetIndexer() cache.Indexer {
	return &providerConfigFilteredCache{
		Indexer:      i.SharedIndexInformer.GetIndexer(),
		filterKey:    i.filterKey,
		filterValue:  i.filterValue,
		allowMissing: i.allowMissing,
	}
}

// RemoveEventHandlers stops all event handlers from receiving events (legacy cleanup).
func (i *ProviderConfigFilteredInformer) RemoveEventHandlers() {
	i.stopped.Store(true)
}

// Cleanup stops all event handlers from receiving events (generic framework cleanup).
func (i *ProviderConfigFilteredInformer) Cleanup() {
	i.RemoveEventHandlers()
}
