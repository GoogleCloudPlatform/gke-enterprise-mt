package filteredinformerv2

import (
	"k8s.io/client-go/tools/cache"
)

// providerConfigFilteredCache implements cache.Store and cache.Indexer with provider config filtering.
type providerConfigFilteredCache struct {
	cache.Indexer
	providerConfigName string
}

// ByIndex returns a list of objects that match the given index name and indexed value.
// The list is filtered to only include objects belonging to the provider config.
func (pc *providerConfigFilteredCache) ByIndex(indexName, indexedValue string) ([]any, error) {
	items, err := pc.Indexer.ByIndex(indexName, indexedValue)
	if err != nil {
		return nil, err
	}
	return providerConfigFilteredList(items, pc.providerConfigName), nil
}

// Index returns a list of objects that match the given index name and indexed value.
// The list is filtered to only include objects belonging to the provider config.
func (pc *providerConfigFilteredCache) Index(indexName string, obj any) ([]any, error) {
	items, err := pc.Indexer.Index(indexName, obj)
	if err != nil {
		return nil, err
	}
	return providerConfigFilteredList(items, pc.providerConfigName), nil
}

// IndexKeys returns a list of keys belonging to the provider config.
func (pc *providerConfigFilteredCache) IndexKeys(indexName, indexedValue string) ([]string, error) {
	keys, err := pc.Indexer.IndexKeys(indexName, indexedValue)
	if err != nil {
		return nil, err
	}

	filteredKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		item, exists, err := pc.Indexer.GetByKey(key)
		if err != nil {
			return nil, err
		}
		if exists && isObjectInProviderConfig(item, pc.providerConfigName) {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return filteredKeys, nil
}

// List returns a list of objects belonging to the provider config.
func (pc *providerConfigFilteredCache) List() []any {
	// Use the index if it exists for a faster lookup.
	items, err := pc.Indexer.ByIndex(providerConfigIndexName, pc.providerConfigName)
	if err == nil {
		return items
	}
	// Fallback to the slower method if the index is not available.
	return providerConfigFilteredList(pc.Indexer.List(), pc.providerConfigName)
}

// ListKeys returns a list of keys belonging to the provider config.
func (pc *providerConfigFilteredCache) ListKeys() []string {
	// Directly query the indexer for keys matching the provider config.
	keys, err := pc.Indexer.IndexKeys(providerConfigIndexName, pc.providerConfigName)
	if err == nil {
		return keys
	}

	// Fallback to the slower method if the index is not available or fails.
	items := pc.List()
	keys = make([]string, 0, len(items))
	for _, item := range items {
		if key, err := cache.MetaNamespaceKeyFunc(item); err == nil {
			keys = append(keys, key)
		}
	}
	return keys
}

// Get returns an object belonging to the provider config.
func (pc *providerConfigFilteredCache) Get(obj any) (item any, exists bool, err error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return nil, false, err
	}
	return pc.GetByKey(key)
}

// GetByKey returns an object belonging to the provider config.
func (pc *providerConfigFilteredCache) GetByKey(key string) (item any, exists bool, err error) {
	item, exists, err = pc.Indexer.GetByKey(key)
	if !exists || err != nil {
		return nil, exists, err
	}
	if isObjectInProviderConfig(item, pc.providerConfigName) {
		return item, true, nil
	}
	return nil, false, nil
}

// LastStoreSyncResourceVersion returns the last resource version from the underlying store.
// Note: This is a new method required by client-go 1.36.
func (pc *providerConfigFilteredCache) LastStoreSyncResourceVersion() string {
	return pc.Indexer.LastStoreSyncResourceVersion()
}

// Bookmark bookmarks the given resource version.
// Note: This is a new method required by client-go 1.36.
func (pc *providerConfigFilteredCache) Bookmark(rv string) {
	pc.Indexer.Bookmark(rv)
}
