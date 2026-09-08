// Package filteredinformerv2 implements informer with provider config filtering.
package filteredinformerv2

import (
	"time"

	"k8s.io/client-go/tools/cache"
)

// ProviderConfigFilteredInformer wraps a SharedIndexInformer to provide a ProviderConfig filtered view.
type ProviderConfigFilteredInformer struct {
	cache.SharedIndexInformer
	providerConfigName string
}

// NewProviderConfigFilteredInformer creates a new ProviderConfigFilteredInformer.
// The providerConfigName is supposed to be ProviderConfig.ObjectMeta.Name.
func NewProviderConfigFilteredInformer(informer cache.SharedIndexInformer, providerConfigName string) cache.SharedIndexInformer {
	indexers := informer.GetIndexer().GetIndexers()
	// Add the index only if the indexers are not nil and the index doesn't already exist.
	if indexers != nil {
		if _, ok := indexers[providerConfigIndexName]; !ok {
			informer.AddIndexers(cache.Indexers{providerConfigIndexName: ProviderConfigIndexFunc})
		}
	}
	return &ProviderConfigFilteredInformer{
		SharedIndexInformer: informer,
		providerConfigName:  providerConfigName,
	}
}

// AddEventHandler adds an event handler that only processes events for the specified ProviderConfig.
func (i *ProviderConfigFilteredInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return return i.SharedIndexInformer.AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: i.providerConfigFilter,
			Handler:    handler,
		},
	)
}

// AddEventHandlerWithResyncPeriod adds an event handler with resync period.
func (i *ProviderConfigFilteredInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return return i.SharedIndexInformer.AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			FilterFunc: i.providerConfigFilter,
			Handler:    handler,
		},
		resyncPeriod,
	)
}

// providerConfigFilter filters objects based on the provider config.
func (i *ProviderConfigFilteredInformer) providerConfigFilter(obj any) bool {
	return isObjectInProviderConfig(obj, i.providerConfigName)
}

// GetStore returns a Store that only stores objects for the specified ProviderConfig.
func (i *ProviderConfigFilteredInformer) GetStore() cache.Store {
	return &providerConfigFilteredCache{
		Indexer:            i.SharedIndexInformer.GetIndexer(),
		providerConfigName: i.providerConfigName,
	}
}

// GetIndexer returns an Indexer that only indexes objects for the specified ProviderConfig.
func (i *ProviderConfigFilteredInformer) GetIndexer() cache.Indexer {
	return &providerConfigFilteredCache{
		Indexer:            i.SharedIndexInformer.GetIndexer(),
		providerConfigName: i.providerConfigName,
	}
}

// RemoveEventHandler removes an event handler.
func (i *ProviderConfigFilteredInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	return i.SharedIndexInformer.RemoveEventHandler(handle)
}

// HasSyncedChecker returns a checker that can be used to check if the informer has synced.
// Note: This is a new method required by client-go 1.36.
func (i *ProviderConfigFilteredInformer) HasSyncedChecker() cache.DoneChecker {
	return i.SharedIndexInformer.HasSyncedChecker()
}
