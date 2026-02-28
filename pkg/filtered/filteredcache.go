/*
Copyright 2026 The Kubernetes Authors.
*/

package filtered

import (
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// FilteredCache implements cache.Store and cache.Indexer with custom filtering.
type FilteredCache struct {
	cache.Indexer
	filterKey    string
	filterValue  string
	allowMissing bool
}

func (obj *FilteredCache) ByIndex(indexName, indexedValue string) ([]interface{}, error) {
	items, err := obj.Indexer.ByIndex(indexName, indexedValue)
	if err != nil {
		return nil, err
	}
	return getFilteredListByValue(items, obj.filterKey, obj.filterValue, obj.allowMissing), nil
}

func (obj *FilteredCache) Index(indexName string, item interface{}) ([]interface{}, error) {
	items, err := obj.Indexer.Index(indexName, item)
	if err != nil {
		return nil, err
	}
	return getFilteredListByValue(items, obj.filterKey, obj.filterValue, obj.allowMissing), nil
}

func (obj *FilteredCache) List() []interface{} {
	return getFilteredListByValue(obj.Indexer.List(), obj.filterKey, obj.filterValue, obj.allowMissing)
}

func (obj *FilteredCache) ListKeys() []string {
	items := obj.List()
	var keys []string
	for _, item := range items {
		if key, err := cache.MetaNamespaceKeyFunc(item); err == nil {
			keys = append(keys, key)
		} else {
			klog.Errorf("ListKeys: failed to get key for item %v: %v", item, err)
		}
	}
	return keys
}

func (obj *FilteredCache) Get(item interface{}) (interface{}, bool, error) {
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		klog.Errorf("Get: failed to get key for item %v: %v", item, err)
		return nil, false, err
	}
	return obj.GetByKey(key)
}

func (obj *FilteredCache) GetByKey(key string) (item interface{}, exists bool, err error) {
	item, exists, err = obj.Indexer.GetByKey(key)
	if !exists || err != nil {
		return nil, exists, err
	}
	if isObjectMatchingValue(item, obj.filterKey, obj.filterValue, obj.allowMissing) {
		return item, true, nil
	}
	return nil, false, nil
}
