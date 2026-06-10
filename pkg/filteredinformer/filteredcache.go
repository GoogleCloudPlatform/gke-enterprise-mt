package filteredinformer

import (
	"google3/third_party/golang/k8s_io/client_go/v/v0_23/tools/cache/cache"
)

// providerConfigFilteredCache implements cache.Store and cache.Indexer with custom filtering.
type providerConfigFilteredCache struct {
	cache.Indexer
	filterKey    string
	filterValue  string
	allowMissing bool
}

// ByIndex returns a list of objects that match the given index name and indexed value.
func (pc *providerConfigFilteredCache) ByIndex(indexName, indexedValue string) ([]any, error) {
	items, err := pc.Indexer.ByIndex(indexName, indexedValue)
	if err != nil {
		return nil, err
	}
	return getFilteredListByValue(items, pc.filterKey, pc.filterValue, pc.allowMissing), nil
}

// Index returns a list of objects that match the given index name and indexed value.
func (pc *providerConfigFilteredCache) Index(indexName string, obj any) ([]any, error) {
	items, err := pc.Indexer.Index(indexName, obj)
	if err != nil {
		return nil, err
	}
	return getFilteredListByValue(items, pc.filterKey, pc.filterValue, pc.allowMissing), nil
}

// List returns a list of objects matching the filter.
func (pc *providerConfigFilteredCache) List() []any {
	// Use the index if it exists for a faster lookup.
	items, err := pc.Indexer.ByIndex(pc.filterKey, pc.filterValue)
	if err == nil {
		return items
	}
	// Fallback to the slower method if the index is not available.
	return getFilteredListByValue(pc.Indexer.List(), pc.filterKey, pc.filterValue, pc.allowMissing)
}

// ListKeys returns a list of keys matching the filter.
func (pc *providerConfigFilteredCache) ListKeys() []string {
	items := pc.List()
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if key, err := cache.MetaNamespaceKeyFunc(item); err == nil {
			keys = append(keys, key)
		}
	}
	return keys
}

// Get returns an object matching the filter.
func (pc *providerConfigFilteredCache) Get(obj any) (item any, exists bool, err error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return nil, false, err
	}
	return pc.GetByKey(key)
}

// GetByKey returns an object matching the filter.
func (pc *providerConfigFilteredCache) GetByKey(key string) (item any, exists bool, err error) {
	item, exists, err = pc.Indexer.GetByKey(key)
	if !exists || err != nil {
		return nil, exists, err
	}
	if isObjectMatchingValue(item, pc.filterKey, pc.filterValue, pc.allowMissing) {
		return item, true, nil
	}
	return nil, false, nil
}
