/*
Copyright 2026 The Kubernetes Authors.
*/

package filtered

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestFilteredCache(t *testing.T) {
	filterKey := "test-key"
	filterValue := "test-value"

	tests := []struct {
		name         string
		allowMissing bool
		items        []*corev1.Pod
		expected     []string
	}{
		{
			name:         "Filter Strict",
			allowMissing: false,
			items: []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "match", Labels: map[string]string{filterKey: filterValue}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "mismatch", Labels: map[string]string{filterKey: "other"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "missing", Labels: map[string]string{}}},
			},
			expected: []string{"match"},
		},
		{
			name:         "Filter Allow Missing",
			allowMissing: true,
			items: []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "match", Labels: map[string]string{filterKey: filterValue}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "mismatch", Labels: map[string]string{filterKey: "other"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "missing", Labels: map[string]string{}}},
			},
			expected: []string{"match", "missing"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
				cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
			})
			fc := &FilteredCache{
				Indexer:      indexer,
				filterKey:    filterKey,
				filterValue:  filterValue,
				allowMissing: tc.allowMissing,
			}

			// Add items
			for _, item := range tc.items {
				err := indexer.Add(item)
				assert.NoError(t, err)
			}

			// Test List
			list := fc.List()
			assert.Len(t, list, len(tc.expected))
			var gotNames []string
			for _, item := range list {
				pod := item.(*corev1.Pod)
				gotNames = append(gotNames, pod.Name)
			}
			assert.ElementsMatch(t, tc.expected, gotNames, "List() returned unexpected items")

			// Test ListKeys
			keys := fc.ListKeys()
			assert.ElementsMatch(t, tc.expected, keys, "ListKeys() returned unexpected keys")

			// Test Get
			for _, item := range tc.items {
				gotItem, exists, err := fc.Get(item)
				assert.NoError(t, err)

				shouldExist := false
				for _, name := range tc.expected {
					if name == item.Name {
						shouldExist = true
						break
					}
				}

				if shouldExist {
					assert.True(t, exists, "Expected item %s to exist", item.Name)
					assert.Equal(t, item, gotItem, "Item %s mismatch", item.Name)
				} else {
					assert.False(t, exists, "Expected item %s to NOT exist", item.Name)
					assert.Nil(t, gotItem, "Expected item %s to be nil", item.Name)
				}
			}

			// Test GetByKey
			for _, item := range tc.items {
				key, _ := cache.MetaNamespaceKeyFunc(item)
				gotItem, exists, err := fc.GetByKey(key)
				assert.NoError(t, err)

				shouldExist := false
				for _, name := range tc.expected {
					if name == item.Name {
						shouldExist = true
						break
					}
				}

				if shouldExist {
					assert.True(t, exists, "Expected key %s to exist", key)
					assert.Equal(t, item, gotItem, "Item for key %s mismatch", key)
				} else {
					assert.False(t, exists, "Expected key %s to NOT exist", key)
					assert.Nil(t, gotItem, "Expected item for key %s to be nil", key)
				}
			}
		})
	}
}

func TestFilteredCache_Index(t *testing.T) {
	filterKey := "test-key"
	filterValue := "test-value"

	// Create a custom indexer
	indexName := "by_label"
	indexFunc := func(obj interface{}) ([]string, error) {
		metaObj, _ := meta.Accessor(obj)
		if val, ok := metaObj.GetLabels()["group"]; ok {
			return []string{val}, nil
		}
		return []string{}, nil
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		indexName: indexFunc,
	})

	fc := &FilteredCache{
		Indexer:      indexer,
		filterKey:    filterKey,
		filterValue:  filterValue,
		allowMissing: false,
	}

	item1 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", Labels: map[string]string{filterKey: filterValue, "group": "A"}}}
	item2 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p2", Labels: map[string]string{filterKey: "wrong", "group": "A"}}}

	err := indexer.Add(item1)
	assert.NoError(t, err)
	err = indexer.Add(item2)
	assert.NoError(t, err)

	// Test ByIndex
	items, err := fc.ByIndex(indexName, "A")
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, item1, items[0])

	// Test Index
	items, err = fc.Index(indexName, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"group": "A"}}})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, item1, items[0])
}


func TestIsObjectInProviderConfig(t *testing.T) {
	tests := []struct {
		name         string
		obj          interface{}
		filterKey    string
		filterValue  string
		allowMissing bool
		want         bool
	}{
		{
			name: "Matching Label",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						providerConfigLabelKey: "config1",
					},
				},
			},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         true,
		},
		{
			name: "Non-Matching Label",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						providerConfigLabelKey: "config2",
					},
				},
			},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         false,
		},
		{
			name: "Missing Label Allowed",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: true,
			want:         true,
		},
		{
			name: "Missing Label Not Allowed",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         false,
		},
		{
			name:         "Invalid Object",
			obj:          "not-an-object",
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isObjectMatchingValue(tc.obj, tc.filterKey, tc.filterValue, tc.allowMissing)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestProviderConfigFilteredList(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod1",
			Labels: map[string]string{
				providerConfigLabelKey: "config1",
			},
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod2",
			Labels: map[string]string{
				providerConfigLabelKey: "config2",
			},
		},
	}
	pod3 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "pod3",
			Labels: map[string]string{},
		},
	}

	tests := []struct {
		name         string
		items        []interface{}
		filterKey    string
		filterValue  string
		allowMissing bool
		want         []interface{}
	}{
		{
			name:         "Filter In Matching Only",
			items:        []interface{}{pod1, pod2, pod3},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         []interface{}{pod1},
		},
		{
			name:         "Filter In Matching and Missing",
			items:        []interface{}{pod1, pod2, pod3},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: true,
			want:         []interface{}{pod1, pod3},
		},
		{
			name:         "Filter In None",
			items:        []interface{}{pod2},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := getFilteredListByValue(tc.items, tc.filterKey, tc.filterValue, tc.allowMissing)
			assert.Equal(t, len(tc.want), len(got))
			for i, item := range got {
				assert.Equal(t, tc.want[i], item)
			}
		})
	}
}