/*
Copyright 2026 The Kubernetes Authors.
*/

package filtered

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/klog/v2"
)

// isObjectMatchingValue checks if an object matches to the specific value.
// If allowMissing is true, objects without the required label are also considered as belonging to the specific value.
func isObjectMatchingValue(obj interface{}, filterKey, filterValue string, allowMissing bool) bool {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		klog.Errorf("isObjectMatchingValue: failed to get meta accessor for object %v: %v", obj, err)
		return false
	}
	val, ok := metaObj.GetLabels()[filterKey]
	return MatchValue(val, ok, filterValue, allowMissing)
}

// MatchValue checks if the specified value matches the expected filter value.
func MatchValue(value string, exists bool, filterValue string, allowMissing bool) bool {
	if allowMissing {
		return !exists || value == filterValue
	}
	return exists && value == filterValue
}

// getFilteredListByValue filters a list of objects by the filter key and value.
func getFilteredListByValue(items []interface{}, filterKey, filterValue string, allowMissing bool) []interface{} {
	var filtered []interface{}
	for _, item := range items {
		if isObjectMatchingValue(item, filterKey, filterValue, allowMissing) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
