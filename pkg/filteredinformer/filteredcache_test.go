package filteredinformer

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestProviderConfigFilteredCache_ByIndex(t *testing.T) {
	testCases := []struct {
		desc                string
		cacheProviderConfig string
		objectsInCache      []any
		queryName           string
		expectedItemNames   []string
	}{
		{
			desc:                "Retrieve items by index in provider config",
			cacheProviderConfig: "cs123456-abc",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Namespace: "cs123456-abc-namespace", Name: "obj1"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Namespace: "cs123456-abc-namespace", Name: "obj2"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs654321-edf"}, Namespace: "cs654321-edf-namespace", Name: "obj1"},
			},
			queryName:         "obj1",
			expectedItemNames: []string{"obj1"},
		},
		{
			desc:                "Retrieve multiple items by index in provider config",
			cacheProviderConfig: "cs123456-abc",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Namespace: "ns1", Name: "obj1"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Namespace: "ns2", Name: "obj1"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs654321-edf"}, Namespace: "ns3", Name: "obj1"},
			},
			queryName:         "obj1",
			expectedItemNames: []string{"obj1", "obj1"},
		},
		{
			desc:                "No items when index key does not match",
			cacheProviderConfig: "cs123456-abc",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Name: "obj1"},
			},
			queryName:         "nonexistent",
			expectedItemNames: []string{},
		},
	}

	indexName := "byName"
	indexers := cache.Indexers{
		indexName: func(obj any) ([]string, error) {
			metaObj, _ := meta.Accessor(obj)
			return []string{metaObj.GetName()}, nil
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, indexers)
			nsCache := &providerConfigFilteredCache{
				Indexer:      indexer,
				filterKey:    providerConfigLabel,
				filterValue:  tc.cacheProviderConfig,
				allowMissing: false,
			}

			for _, obj := range tc.objectsInCache {
				indexer.Add(obj)
			}

			items, err := nsCache.ByIndex(indexName, tc.queryName)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(items) != len(tc.expectedItemNames) {
				t.Errorf("Expected %d items, got %d", len(tc.expectedItemNames), len(items))
			}

			gotNames := make([]string, 0, len(items))
			for _, item := range items {
				metaObj, _ := meta.Accessor(item)
				gotNames = append(gotNames, metaObj.GetName())
			}
			sort.Strings(gotNames)
			expected := make([]string, len(tc.expectedItemNames))
			copy(expected, tc.expectedItemNames)
			sort.Strings(expected)
			if !reflect.DeepEqual(gotNames, expected) {
				t.Errorf("Expected item names %v, got %v", expected, gotNames)
			}
		})
	}
}

func TestProviderConfigFilteredCache_Index(t *testing.T) {
	testCases := []struct {
		desc                string
		cacheProviderConfig string
		objectsInCache      []any
		queryObjName        string
		expectedItemNames   []string
		expectedErr         error
	}{
		{
			desc:                "Retrieve items by index in provider config",
			cacheProviderConfig: "cs123456-abc",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Namespace: "cs123456-abc-namespace", Name: "obj1"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Namespace: "cs123456-abc-namespace", Name: "obj2"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs654321-edf"}, Namespace: "cs654321-edf-namespace", Name: "obj1"},
			},
			queryObjName:      "obj1",
			expectedItemNames: []string{"obj1"},
		},
		{
			desc:                "Retrieve multiple items by index in provider config",
			cacheProviderConfig: "cs123456-abc",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Namespace: "ns1", Name: "obj1"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Namespace: "ns2", Name: "obj1"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs654321-edf"}, Namespace: "ns3", Name: "obj1"},
			},
			queryObjName:      "obj1",
			expectedItemNames: []string{"obj1", "obj1"},
		},
		{
			desc:                "No items when index key does not match",
			cacheProviderConfig: "cs123456-abc",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Name: "obj1"},
			},
			queryObjName:      "nonexistent",
			expectedItemNames: []string{},
		},
		{
			desc:                "Error from indexer",
			cacheProviderConfig: "cs123456-abc",
			queryObjName:        "obj1",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "cs123456-abc"}, Namespace: "cs123456-abc-namespace", Name: "obj1"},
			},
			expectedErr: errors.New("synthetic index error"),
		},
	}

	indexName := "byName"
	indexFunc := func(obj any) ([]string, error) {
		metaObj, _ := meta.Accessor(obj)
		return []string{metaObj.GetName()}, nil
	}
	indexers := cache.Indexers{indexName: indexFunc}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			realIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, indexers)
			var indexer cache.Indexer = realIndexer
			if tc.expectedErr != nil {
				indexer = &mockIndexer{Indexer: realIndexer, indexErr: tc.expectedErr}
			}
			nsCache := &providerConfigFilteredCache{
				Indexer:      indexer,
				filterKey:    providerConfigLabel,
				filterValue:  tc.cacheProviderConfig,
				allowMissing: false,
			}

			for _, obj := range tc.objectsInCache {
				realIndexer.Add(obj)
			}
			queryObj := &v1.ObjectMeta{Name: tc.queryObjName}

			items, err := nsCache.Index(indexName, queryObj)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("Expected error %v, got nil", tc.expectedErr)
				}
				if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', got '%v'", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(items) != len(tc.expectedItemNames) {
				t.Errorf("Expected %d items, got %d", len(tc.expectedItemNames), len(items))
			}

			gotNames := make([]string, 0, len(items))
			for _, item := range items {
				metaObj, _ := meta.Accessor(item)
				gotNames = append(gotNames, metaObj.GetName())
			}
			sort.Strings(gotNames)
			expected := make([]string, len(tc.expectedItemNames))
			copy(expected, tc.expectedItemNames)
			sort.Strings(expected)
			if !reflect.DeepEqual(gotNames, expected) {
				t.Errorf("Expected item names %v, got %v", expected, gotNames)
			}
		})
	}
}

func TestProviderConfigFilteredCache_List(t *testing.T) {
	testCases := []struct {
		desc                string
		cacheProviderConfig string
		objectsInCache      []any
		expectedItemNames   []string
	}{
		{
			desc:                "List items in the provider config",
			cacheProviderConfig: "p123456-abc",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}, Name: "obj1"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p654321-edf"}, Name: "obj2"},
			},
			expectedItemNames: []string{"obj1"},
		},
		{
			desc:                "List no items when provider config has no objects",
			cacheProviderConfig: "p123456-abc",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p654321-edf"}, Name: "obj1"},
			},
			expectedItemNames: []string{},
		},
	}

	indexTestCases := []struct {
		name     string
		indexers cache.Indexers
	}{
		{
			name:     "with provider config index",
			indexers: cache.Indexers{providerConfigLabel: NewLabelIndexFunc(providerConfigLabel)},
		},
		{
			name:     "without provider config index",
			indexers: nil,
		},
	}

	for _, indexTC := range indexTestCases {
		indexTC := indexTC
		t.Run(indexTC.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range testCases {
				tc := tc
				t.Run(tc.desc, func(t *testing.T) {
					t.Parallel()
					indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, indexTC.indexers)
					nsCache := &providerConfigFilteredCache{
						Indexer:      indexer,
						filterKey:    providerConfigLabel,
						filterValue:  tc.cacheProviderConfig,
						allowMissing: false,
					}

					for _, obj := range tc.objectsInCache {
						indexer.Add(obj)
					}

					items := nsCache.List()
					if len(items) != len(tc.expectedItemNames) {
						t.Errorf("Expected %d items, got %d", len(tc.expectedItemNames), len(items))
					}

					gotNames := make([]string, 0, len(items))
					for _, item := range items {
						metaObj, _ := meta.Accessor(item)
						gotNames = append(gotNames, metaObj.GetName())
					}
					sort.Strings(gotNames)
					// Create a copy to avoid modifying the test case data.
					expected := make([]string, len(tc.expectedItemNames))
					copy(expected, tc.expectedItemNames)
					sort.Strings(expected)
					if !reflect.DeepEqual(gotNames, expected) {
						t.Errorf("Expected item names %v, got %v", expected, gotNames)
					}
				})
			}
		})
	}
}

func TestProviderConfigFilteredCache_ListKeys(t *testing.T) {
	testCases := []struct {
		desc                string
		cacheProviderConfig string
		objectsInCache      []any
		expectedKeys        []string
	}{
		{
			desc:                "List keys for objects in provider config",
			cacheProviderConfig: "p123456-abc",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}, Namespace: "ns1", Name: "obj1"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p654321-edf"}, Namespace: "ns2", Name: "obj2"},
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}, Namespace: "ns1", Name: "obj3"},
			},
			expectedKeys: []string{"ns1/obj1", "ns1/obj3"},
		},
		{
			desc:                "List no keys when no objects in provider config",
			cacheProviderConfig: "p123456-abc",
			objectsInCache: []any{
				&v1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p654321-edf"}, Name: "obj1"},
			},
			expectedKeys: []string{},
		},
		{
			desc:                "List no keys when cache is empty",
			cacheProviderConfig: "p123456-abc",
			objectsInCache:      []any{},
			expectedKeys:        []string{},
		},
	}

	indexTestCases := []struct {
		name     string
		indexers cache.Indexers
	}{
		{
			name:     "with provider config index",
			indexers: cache.Indexers{providerConfigLabel: NewLabelIndexFunc(providerConfigLabel)},
		},
		{
			name:     "without provider config index",
			indexers: nil,
		},
	}

	for _, indexTC := range indexTestCases {
		indexTC := indexTC
		t.Run(indexTC.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range testCases {
				tc := tc
				t.Run(tc.desc, func(t *testing.T) {
					t.Parallel()
					indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, indexTC.indexers)
					nsCache := &providerConfigFilteredCache{
						Indexer:      indexer,
						filterKey:    providerConfigLabel,
						filterValue:  tc.cacheProviderConfig,
						allowMissing: false,
					}
					for _, obj := range tc.objectsInCache {
						indexer.Add(obj)
					}
					keys := nsCache.ListKeys()
					sort.Strings(keys)
					expected := make([]string, len(tc.expectedKeys))
					copy(expected, tc.expectedKeys)
					sort.Strings(expected)
					if !reflect.DeepEqual(keys, expected) {
						t.Errorf("Expected keys %v, got %v", expected, keys)
					}
				})
			}
		})
	}
}

// mockIndexer is a wrapper around a cache.Indexer that can be configured to return
// errors for testing.
type mockIndexer struct {
	cache.Indexer
	getByKeyErr    error
	getByKeyExists bool
	indexErr       error
}

func (m *mockIndexer) Index(indexName string, obj any) ([]any, error) {
	if m.indexErr != nil {
		return nil, m.indexErr
	}
	return m.Indexer.Index(indexName, obj)
}

func (m *mockIndexer) GetByKey(key string) (any, bool, error) {
	item, exists, err := m.Indexer.GetByKey(key)
	if err != nil {
		// This shouldn't happen with the real indexer used in tests, but handle it just in case.
		return item, exists, err
	}
	if m.getByKeyErr != nil {
		// Return the item and exists from the underlying indexer, but with the mock error.
		return item, m.getByKeyExists, m.getByKeyErr
	}
	return item, exists, nil
}

func TestProviderConfigFilteredCache_GetByKey(t *testing.T) {
	testCases := []struct {
		desc                string
		cacheProviderConfig string
		queryKey            string
		objectsInCache      []any
		expectedExist       bool
		mockIndexerExists   bool
		expectedName        string
		expectedErr         error
	}{
		{
			desc:                "Get existing item by key in provider config",
			cacheProviderConfig: "p123456-abc",
			queryKey:            "p123456-abc-namespace/obj1",
			objectsInCache: []any{&v1.ObjectMeta{
				Labels:    map[string]string{providerConfigLabel: "p123456-abc"},
				Namespace: "p123456-abc-namespace",
				Name:      "obj1",
			}},
			expectedExist: true,
			expectedName:  "obj1",
		},
		{
			desc:                "Item exists but in different provider config",
			cacheProviderConfig: "p123456-abc",
			queryKey:            "p654321-edf-namespace/obj1",
			objectsInCache: []any{&v1.ObjectMeta{
				Labels:    map[string]string{providerConfigLabel: "p654321-edf"},
				Namespace: "p654321-edf-namespace",
				Name:      "obj1",
			}},
			expectedExist: false,
		},
		{
			desc:                "Item does not exist",
			cacheProviderConfig: "p123456-abc",
			objectsInCache: []any{&v1.ObjectMeta{
				Labels:    map[string]string{providerConfigLabel: "p123456-abc"},
				Namespace: "p123456-abc-namespace",
				Name:      "obj1",
			}},
			queryKey:      "p123456-abc-namespace/obj2",
			expectedExist: false,
		},
		{
			desc:                "Error from indexer, exists true",
			cacheProviderConfig: "p123456-abc",
			queryKey:            "p123456-abc-namespace/obj1",
			objectsInCache: []any{&v1.ObjectMeta{
				Labels:    map[string]string{providerConfigLabel: "p123456-abc"},
				Namespace: "p123456-abc-namespace",
				Name:      "obj1",
			}},
			expectedExist:     true,
			mockIndexerExists: true,
			expectedErr:       errors.New("synthetic getbykey error"),
		},
		{
			desc:                "Error from indexer, exists false",
			cacheProviderConfig: "p123456-abc",
			queryKey:            "p123456-abc-namespace/obj2",
			objectsInCache: []any{&v1.ObjectMeta{
				Labels:    map[string]string{providerConfigLabel: "p123456-abc"},
				Namespace: "p123456-abc-namespace",
				Name:      "obj1",
			}},
			expectedExist:     false,
			mockIndexerExists: false,
			expectedErr:       errors.New("synthetic getbykey error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			realIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, nil)
			var indexer cache.Indexer = realIndexer
			if tc.expectedErr != nil {
				indexer = &mockIndexer{Indexer: realIndexer, getByKeyErr: tc.expectedErr, getByKeyExists: tc.mockIndexerExists}
			}
			nsCache := &providerConfigFilteredCache{
				Indexer:      indexer,
				filterKey:    providerConfigLabel,
				filterValue:  tc.cacheProviderConfig,
				allowMissing: false,
			}

			for _, obj := range tc.objectsInCache {
				realIndexer.Add(obj)
			}

			item, exists, err := nsCache.GetByKey(tc.queryKey)
			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("Expected error %v, got nil", tc.expectedErr)
				}
				if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', got '%v'", tc.expectedErr, err)
				}
				if exists != tc.expectedExist {
					t.Errorf("Expected exists to be %v, got %v", tc.expectedExist, exists)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if exists != tc.expectedExist {
				t.Errorf("Expected exists to be %v, got %v", tc.expectedExist, exists)
			}
			if exists && item != nil {
				metaObj, _ := meta.Accessor(item)
				if metaObj.GetName() != tc.expectedName {
					t.Errorf("Expected item name %s, got %s", tc.expectedName, metaObj.GetName())
				}
			}
		})
	}
}

func TestProviderConfigFilteredCache_Get(t *testing.T) {
	getByKeyErr := errors.New("synthetic getbykey error")
	keyFuncErr := errors.New("object has no meta: object does not implement the Object interfaces")

	testCases := []struct {
		desc                string
		cacheProviderConfig string
		queryObj            any
		objectsInCache      []any
		expectedExist       bool
		expectedName        string
		expectedErr         error
	}{
		{
			desc:                "Get existing item in provider config",
			cacheProviderConfig: "p123456-abc",
			queryObj: &v1.ObjectMeta{
				Namespace: "p123456-abc-namespace",
				Name:      "obj1",
			},
			objectsInCache: []any{&v1.ObjectMeta{
				Labels:    map[string]string{providerConfigLabel: "p123456-abc"},
				Namespace: "p123456-abc-namespace",
				Name:      "obj1",
			}},
			expectedExist: true,
			expectedName:  "obj1",
		},
		{
			desc:                "Item exists but in different provider config",
			cacheProviderConfig: "p123456-abc",
			queryObj: &v1.ObjectMeta{
				Namespace: "p654321-edf-namespace",
				Name:      "obj1",
			},
			objectsInCache: []any{&v1.ObjectMeta{
				Labels:    map[string]string{providerConfigLabel: "p654321-edf"},
				Namespace: "p654321-edf-namespace",
				Name:      "obj1",
			}},
			expectedExist: false,
		},
		{
			desc:                "Item does not exist",
			cacheProviderConfig: "p123456-abc",
			queryObj: &v1.ObjectMeta{
				Namespace: "p123456-abc-namespace",
				Name:      "obj2",
			},
			objectsInCache: []any{&v1.ObjectMeta{
				Labels:    map[string]string{providerConfigLabel: "p123456-abc"},
				Namespace: "p123456-abc-namespace",
				Name:      "obj1",
			}},
			expectedExist: false,
		},
		{
			desc:                "Error from key func",
			cacheProviderConfig: "p123456-abc",
			queryObj:            "a string is not a valid object",
			expectedExist:       false,
			expectedErr:         keyFuncErr,
		},
		{
			desc:                "Error from GetByKey",
			cacheProviderConfig: "p123456-abc",
			queryObj: &v1.ObjectMeta{
				Namespace: "p123456-abc-namespace",
				Name:      "obj1",
			},
			objectsInCache: []any{&v1.ObjectMeta{
				Labels:    map[string]string{providerConfigLabel: "p123456-abc"},
				Namespace: "p123456-abc-namespace",
				Name:      "obj1",
			}},
			expectedExist: true,
			expectedErr:   getByKeyErr,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			realIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, nil)
			var indexer cache.Indexer = realIndexer
			if tc.expectedErr == getByKeyErr {
				indexer = &mockIndexer{Indexer: realIndexer, getByKeyErr: tc.expectedErr, getByKeyExists: tc.expectedExist}
			}
			nsCache := &providerConfigFilteredCache{
				Indexer:      indexer,
				filterKey:    providerConfigLabel,
				filterValue:  tc.cacheProviderConfig,
				allowMissing: false,
			}

			for _, obj := range tc.objectsInCache {
				realIndexer.Add(obj)
			}

			item, exists, err := nsCache.Get(tc.queryObj)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("Expected error %v, got nil", tc.expectedErr)
				}
				if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', got '%v'", tc.expectedErr, err)
				}
				if exists != tc.expectedExist {
					t.Errorf("Expected exists to be %v, got %v", tc.expectedExist, exists)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if exists != tc.expectedExist {
				t.Errorf("Expected exists to be %v, got %v", tc.expectedExist, exists)
			}
			if exists && item != nil {
				metaObj, _ := meta.Accessor(item)
				if metaObj.GetName() != tc.expectedName {
					t.Errorf("Expected item name %s, got %s", tc.expectedName, metaObj.GetName())
				}
			}
		})
	}
}
