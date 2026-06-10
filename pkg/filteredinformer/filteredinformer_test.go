// Package filteredinformer implements informer with provider config filtering.
package filteredinformer

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// TestNewProviderConfigFilteredInformer verifies that NewProviderConfigFilteredInformer
// adds the provider config index and is idempotent.
func TestNewProviderConfigFilteredInformer(t *testing.T) {
	t.Run("with non-nil indexer", func(t *testing.T) {
		sharedInformer := cache.NewSharedIndexInformer(nil, &corev1.Pod{}, 0, cache.Indexers{})
		// Initially, no indexer.
		indexers := sharedInformer.GetIndexer().GetIndexers()
		if _, ok := indexers[providerConfigLabel]; ok {
			t.Fatalf("Indexer %q should not exist initially", providerConfigLabel)
		}
		// First call should add the indexer.
		_ = NewProviderConfigFilteredInformer(sharedInformer, "test-provider-config-1")
		indexers = sharedInformer.GetIndexer().GetIndexers()
		if _, ok := indexers[providerConfigLabel]; !ok {
			t.Errorf("Indexer %q should have been added", providerConfigLabel)
		}
		// Second call should not fail and the indexer should still be there.
		_ = NewProviderConfigFilteredInformer(sharedInformer, "test-provider-config-2")
		indexers = sharedInformer.GetIndexer().GetIndexers()
		if _, ok := indexers[providerConfigLabel]; !ok {
			t.Errorf("Indexer %q should still be present after second call", providerConfigLabel)
		}
	})

	t.Run("with nil indexer", func(t *testing.T) {
		sharedInformer := cache.NewSharedIndexInformer(nil, &corev1.Pod{}, 0, nil)
		// Should not panic.
		_ = NewProviderConfigFilteredInformer(sharedInformer, "test-provider-config-1")

		indexers := sharedInformer.GetIndexer().GetIndexers()
		if indexers != nil {
			t.Fatal("Indexers should be nil, but got non-nil")
		}
	})
}

// TestFilteredInformer_AddEventHandler verifies that the
// filteredinformer.AddEventHandler method does not return an error.
func TestFilteredInformer_AddEventHandler(t *testing.T) {
	sharedInformer := cache.NewSharedIndexInformer(nil, &corev1.Pod{}, 0, cache.Indexers{})
	filteredinformer := NewProviderConfigFilteredInformer(sharedInformer, "test-provider-config")

	handler := cache.ResourceEventHandlerFuncs{}

	filteredinformer.AddEventHandler(handler)
}

// TestFilteredInformer_AddEventHandlerWithResyncPeriod verifies that the
// namespacedinformer.AddEventHandlerWithResyncPeriod method does not return an
// error.
func TestFilteredInformer_AddEventHandlerWithResyncPeriod(t *testing.T) {
	testCases := []struct {
		desc               string
		providerConfigName string
		resyncPeriod       time.Duration
	}{
		{
			desc:               "Add event handler with resync period",
			providerConfigName: "test-provider-config",
			resyncPeriod:       time.Minute,
		},
		{
			desc:               "Add event handler with zero resync period",
			providerConfigName: "test-provider-config",
			resyncPeriod:       0,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			sharedInformer := cache.NewSharedIndexInformer(nil, &corev1.Pod{}, 0, cache.Indexers{})
			filteredinformer := NewProviderConfigFilteredInformer(sharedInformer, tc.providerConfigName)

			handler := cache.ResourceEventHandlerFuncs{}
			filteredinformer.AddEventHandlerWithResyncPeriod(handler, tc.resyncPeriod)
		})
	}
}

// mockEventHandler tracks the objects it receives for testing.
type mockEventHandler struct {
	addCalls    int
	updateCalls int
	deleteCalls int
}

func (m *mockEventHandler) OnAdd(obj any) {
	m.addCalls++
}

func (m *mockEventHandler) OnUpdate(oldObj, newObj any) {
	m.updateCalls++
}

func (m *mockEventHandler) OnDelete(obj any) {
	m.deleteCalls++
}

// fakeInformer implements a fake SharedIndexInformer for testing.
type fakeInformer struct {
	cache.SharedIndexInformer
	handler  cache.ResourceEventHandler
	indexers cache.Indexers
	indexer  cache.Indexer
}

func (f *fakeInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	f.handler = handler
}

func (f *fakeInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) {
	f.handler = handler
}

func (f *fakeInformer) GetIndexer() cache.Indexer {
	// The indexer needs to be a real one for the test to work.
	if f.indexer == nil {
		f.indexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, f.indexers)
	}
	return f.indexer
}

func (f *fakeInformer) AddIndexers(indexers cache.Indexers) error {
	if f.indexers == nil {
		f.indexers = cache.Indexers{}
	}
	for name, fn := range indexers {
		f.indexers[name] = fn
	}
	return nil
}

// TestProviderConfigFilteredInformer_EventHandlerFiltering verifies that the event handler
// filtering logic works as expected.
func TestProviderConfigFilteredInformer_EventHandlerFiltering(t *testing.T) {
	providerConfigName := "p123456-abc"

	matchingObj := &metav1.ObjectMeta{
		Labels: map[string]string{providerConfigLabel: providerConfigName},
		Name:   "matching-obj",
	}
	nonMatchingObj := &metav1.ObjectMeta{
		Labels: map[string]string{providerConfigLabel: "p654321-def"},
		Name:   "non-matching-obj",
	}
	objWithoutLabel := &metav1.ObjectMeta{
		Name: "no-label-obj",
	}

	testCases := []struct {
		desc                string
		event               func(h cache.ResourceEventHandler)
		expectedAddCalls    int
		expectedUpdateCalls int
		expectedDeleteCalls int
	}{
		{
			desc: "OnAdd with matching object",
			event: func(h cache.ResourceEventHandler) {
				h.OnAdd(matchingObj)
			},
			expectedAddCalls: 1,
		},
		{
			desc: "OnAdd with non-matching object",
			event: func(h cache.ResourceEventHandler) {
				h.OnAdd(nonMatchingObj)
			},
		},
		{
			desc: "OnAdd with object without label",
			event: func(h cache.ResourceEventHandler) {
				h.OnAdd(objWithoutLabel)
			},
		},
		{
			desc: "OnUpdate with matching new object",
			event: func(h cache.ResourceEventHandler) {
				h.OnUpdate(matchingObj, matchingObj)
			},
			expectedUpdateCalls: 1,
		},
		{
			desc: "OnUpdate with non-matching new object",
			event: func(h cache.ResourceEventHandler) {
				h.OnUpdate(nonMatchingObj, nonMatchingObj)
			},
		},
		{
			desc: "OnUpdate with new object without label",
			event: func(h cache.ResourceEventHandler) {
				h.OnUpdate(objWithoutLabel, objWithoutLabel)
			},
		},
		{
			desc: "OnDelete with matching object",
			event: func(h cache.ResourceEventHandler) {
				h.OnDelete(matchingObj)
			},
			expectedDeleteCalls: 1,
		},
		{
			desc: "OnDelete with non-matching object",
			event: func(h cache.ResourceEventHandler) {
				h.OnDelete(nonMatchingObj)
			},
		},
		{
			desc: "OnDelete with object without label",
			event: func(h cache.ResourceEventHandler) {
				h.OnDelete(objWithoutLabel)
			},
		},
		{
			desc: "OnDelete with matching DeletedFinalStateUnknown",
			event: func(h cache.ResourceEventHandler) {
				h.OnDelete(cache.DeletedFinalStateUnknown{Key: "some-key", Obj: matchingObj})
			},
			expectedDeleteCalls: 1,
		},
		{
			desc: "OnDelete with non-matching DeletedFinalStateUnknown",
			event: func(h cache.ResourceEventHandler) {
				h.OnDelete(cache.DeletedFinalStateUnknown{Key: "some-key", Obj: nonMatchingObj})
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			fake := &fakeInformer{}
			informer := NewProviderConfigFilteredInformer(fake, providerConfigName)

			// The mock handler is wrapped by the FilteringResourceEventHandler, so we
			// add it to the informer and then trigger the wrapped handler.
			mockHandler := &mockEventHandler{}
			informer.AddEventHandler(mockHandler)
			tc.event(fake.handler)

			if mockHandler.addCalls != tc.expectedAddCalls {
				t.Errorf("OnAdd calls: got %d, want %d", mockHandler.addCalls, tc.expectedAddCalls)
			}
			if mockHandler.updateCalls != tc.expectedUpdateCalls {
				t.Errorf("OnUpdate calls: got %d, want %d", mockHandler.updateCalls, tc.expectedUpdateCalls)
			}
			if mockHandler.deleteCalls != tc.expectedDeleteCalls {
				t.Errorf("OnDelete calls: got %d, want %d", mockHandler.deleteCalls, tc.expectedDeleteCalls)
			}
		})
	}
}

// TestProviderConfigFilteredInformer_RemoveEventHandlers verifies that event handlers do not receive events after Stop() is called.
func TestProviderConfigFilteredInformer_RemoveEventHandlers(t *testing.T) {
	providerConfigName := "p123456-abc"

	matchingObj := &metav1.ObjectMeta{
		Labels: map[string]string{providerConfigLabel: providerConfigName},
		Name:   "matching-obj",
	}

	fake := &fakeInformer{}
	informer := NewProviderConfigFilteredInformer(fake, providerConfigName)

	mockHandler := &mockEventHandler{}
	informer.AddEventHandler(mockHandler)

	// Events should be received before stop.
	fake.handler.OnAdd(matchingObj)
	if mockHandler.addCalls != 1 {
		t.Fatalf("OnAdd calls before stop: got %d, want 1", mockHandler.addCalls)
	}

	// Remove event handlers.
	informer.(*ProviderConfigFilteredInformer).RemoveEventHandlers()

	// Events should not be received after stop.
	fake.handler.OnAdd(matchingObj)
	if mockHandler.addCalls != 1 {
		t.Errorf("OnAdd call count after stop: got %d, want 1 (no new events should be received)", mockHandler.addCalls)
	}
	fake.handler.OnUpdate(matchingObj, matchingObj)
	if mockHandler.updateCalls != 0 {
		t.Errorf("OnUpdate call count after stop: got %d, want 0", mockHandler.updateCalls)
	}
	fake.handler.OnDelete(matchingObj)
	if mockHandler.deleteCalls != 0 {
		t.Errorf("OnDelete call count after stop: got %d, want 0", mockHandler.deleteCalls)
	}
}

// TestProviderConfigFilteredCache verifies that the GetStore and GetIndexer methods
// return a cache that correctly filters objects based on the provider config.
func TestProviderConfigFilteredCache(t *testing.T) {
	providerConfigName1 := "p1"
	providerConfigName2 := "p2"

	matchingObj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:   "matching-obj",
		Labels: map[string]string{providerConfigLabel: providerConfigName1},
	}}
	nonMatchingObj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:   "non-matching-obj",
		Labels: map[string]string{providerConfigLabel: providerConfigName2},
	}}
	objWithoutLabel := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "no-label-obj",
	}}

	fakeInformer := &fakeInformer{
		indexers: cache.Indexers{providerConfigLabel: NewLabelIndexFunc(providerConfigLabel)},
	}
	indexer := fakeInformer.GetIndexer()
	indexer.Add(matchingObj)
	indexer.Add(nonMatchingObj)
	indexer.Add(objWithoutLabel)

	informer := NewProviderConfigFilteredInformer(fakeInformer, providerConfigName1)
	store := informer.GetStore()
	idx := informer.GetIndexer()

	t.Run("List", func(t *testing.T) {
		items := store.List()
		if len(items) != 1 {
			t.Fatalf("List() returned %d items, want 1", len(items))
		}
		if pod, ok := items[0].(*corev1.Pod); !ok || pod.Name != matchingObj.Name {
			t.Errorf("List() returned wrong item: got %v, want %s", items[0], matchingObj.Name)
		}
	})

	t.Run("ListKeys", func(t *testing.T) {
		keys := store.ListKeys()
		if len(keys) != 1 {
			t.Fatalf("ListKeys() returned %d keys, want 1", len(keys))
		}
		expectedKey, _ := cache.MetaNamespaceKeyFunc(matchingObj)
		if keys[0] != expectedKey {
			t.Errorf("ListKeys() returned wrong key: got %s, want %s", keys[0], expectedKey)
		}
	})

	t.Run("Get", func(t *testing.T) {
		// Test getting a matching object.
		item, exists, err := store.Get(matchingObj)
		if err != nil {
			t.Fatalf("Get(matchingObj) returned an error: %v", err)
		}
		if !exists {
			t.Error("Get(matchingObj) should exist")
		}
		if pod, ok := item.(*corev1.Pod); !ok || pod.Name != matchingObj.Name {
			t.Errorf("Get(matchingObj) returned wrong item: got %v, want %s", item, matchingObj.Name)
		}

		// Test getting a non-matching object.
		_, exists, err = store.Get(nonMatchingObj)
		if err != nil {
			t.Fatalf("Get(nonMatchingObj) returned an error: %v", err)
		}
		if exists {
			t.Error("Get(nonMatchingObj) should not exist")
		}
	})

	t.Run("GetByKey", func(t *testing.T) {
		matchingKey, _ := cache.MetaNamespaceKeyFunc(matchingObj)
		nonMatchingKey, _ := cache.MetaNamespaceKeyFunc(nonMatchingObj)

		// Test getting a matching object by key.
		item, exists, err := store.GetByKey(matchingKey)
		if err != nil {
			t.Fatalf("GetByKey(matchingKey) returned an error: %v", err)
		}
		if !exists {
			t.Error("GetByKey(matchingKey) should exist")
		}
		if pod, ok := item.(*corev1.Pod); !ok || pod.Name != matchingObj.Name {
			t.Errorf("GetByKey(matchingKey) returned wrong item: got %v, want %s", item, matchingObj.Name)
		}

		// Test getting a non-matching object by key.
		_, exists, err = store.GetByKey(nonMatchingKey)
		if err != nil {
			t.Fatalf("GetByKey(nonMatchingKey) returned an error: %v", err)
		}
		if exists {
			t.Error("GetByKey(nonMatchingKey) should not exist")
		}
	})

	t.Run("ByIndex", func(t *testing.T) {
		// Test with a matching provider config.
		items, err := idx.ByIndex(providerConfigLabel, providerConfigName1)
		if err != nil {
			t.Fatalf("ByIndex(matching) returned an error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("ByIndex(matching) returned %d items, want 1", len(items))
		}
		if pod, ok := items[0].(*corev1.Pod); !ok || pod.Name != matchingObj.Name {
			t.Errorf("ByIndex(matching) returned wrong item: got %v, want %s", items[0], matchingObj.Name)
		}

		// Test with a non-matching provider config.
		items, err = idx.ByIndex(providerConfigLabel, providerConfigName2)
		if err != nil {
			t.Fatalf("ByIndex(non-matching) returned an error: %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("ByIndex(non-matching) returned %d items, want 0, got: %v", len(items), items)
		}
	})
}
