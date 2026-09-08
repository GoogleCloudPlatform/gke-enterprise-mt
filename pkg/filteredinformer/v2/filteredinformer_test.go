package filteredinformerv2

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
		if _, ok := indexers[providerConfigIndexName]; ok {
			t.Fatalf("Indexer %q should not exist initially", providerConfigIndexName)
		}
		// First call should add the indexer.
		_ = NewProviderConfigFilteredInformer(sharedInformer, "test-provider-config-1")
		indexers = sharedInformer.GetIndexer().GetIndexers()
		if _, ok := indexers[providerConfigIndexName]; !ok {
			t.Errorf("Indexer %q should have been added", providerConfigIndexName)
		}
		// Second call should not fail and the indexer should still be there.
		_ = NewProviderConfigFilteredInformer(sharedInformer, "test-provider-config-2")
		indexers = sharedInformer.GetIndexer().GetIndexers()
		if _, ok := indexers[providerConfigIndexName]; !ok {
			t.Errorf("Indexer %q should still be present after second call", providerConfigIndexName)
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

	if _, err := filteredinformer.AddEventHandler(handler); err != nil {
		t.Errorf("AddEventHandler(%v) returned an unexpected error: %v", handler, err)
	}
}

// TestFilteredInformer_AddEventHandlerWithResyncPeriod verifies that the
// namespacedinformer.AddEventHandlerWithResyncPeriod method does not return an
// error.
func TestFilteredInformer_AddEventHandlerWithResyncPeriod(t *testing.T) {
	testCases := []struct {
		desc         string
		resyncPeriod time.Duration
	}{
		{
			desc:         "Add event handler with resync period",
			resyncPeriod: time.Minute,
		},
		{
			desc:         "Add event handler with zero resync period",
			resyncPeriod: 0,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			sharedInformer := cache.NewSharedIndexInformer(nil, &corev1.Pod{}, 0, cache.Indexers{})
			filteredinformer := NewProviderConfigFilteredInformer(sharedInformer, "test-provider-config")

			handler := cache.ResourceEventHandlerFuncs{}
			if _, err := filteredinformer.AddEventHandlerWithResyncPeriod(handler, tc.resyncPeriod); err != nil {
				t.Errorf("AddEventHandlerWithResyncPeriod(%v, %v) returned an unexpected error: %v", handler, tc.resyncPeriod, err)
			}
		})
	}
}

// TestFilteredInformer_AddEventHandlerWithOptions verifies that the
// filteredinformer.AddEventHandlerWithOptions method passes options correctly.
func TestFilteredInformer_AddEventHandlerWithOptions(t *testing.T) {
	fake := &fakeInformer{}
	filteredinformer := NewProviderConfigFilteredInformer(fake, "test-provider-config")

	handler := cache.ResourceEventHandlerFuncs{}
	resyncPeriod := time.Minute
	options := cache.HandlerOptions{
		ResyncPeriod: &resyncPeriod,
	}

	if _, err := filteredinformer.AddEventHandlerWithOptions(handler, options); err != nil {
		t.Errorf("AddEventHandlerWithOptions returned unexpected error: %v", err)
	}

	if fake.handler == nil {
		t.Fatal("Expected handler to be set on fake informer")
	}
	
	// Verify options were passed through
	if fake.options.ResyncPeriod == nil || *fake.options.ResyncPeriod != resyncPeriod {
		t.Errorf("Expected ResyncPeriod to be %v, got %v", resyncPeriod, fake.options.ResyncPeriod)
	}
}

// mockEventHandler tracks the objects it receives for testing.
type mockEventHandler struct {
	addCalls    int
	updateCalls int
	deleteCalls int
}

func (m *mockEventHandler) OnAdd(obj any, isInInitialList bool) {
	m.addCalls++
}

func (m *mockEventHandler) OnUpdate(oldObj, newObj any) {
	m.updateCalls++
}

func (m *mockEventHandler) OnDelete(obj any) {
	m.deleteCalls++
}

type fakeHandle struct{}

func (f *fakeHandle) HasSynced() bool {
	return true
}

func (f *fakeHandle) HasSyncedChecker() cache.DoneChecker {
	return nil
}

// fakeInformer implements a fake SharedIndexInformer for testing.
type fakeInformer struct {
	cache.SharedIndexInformer
	handler       cache.ResourceEventHandler
	options       cache.HandlerOptions
	indexers      cache.Indexers
	indexer       cache.Indexer
	removedHandle cache.ResourceEventHandlerRegistration
	removeErr     error
}

func (f *fakeInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	f.handler = handler
	return &fakeHandle{}, nil
}

func (f *fakeInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	f.handler = handler
	return &fakeHandle{}, nil
}

func (f *fakeInformer) AddEventHandlerWithOptions(handler cache.ResourceEventHandler, options cache.HandlerOptions) (cache.ResourceEventHandlerRegistration, error) {
	f.handler = handler
	f.options = options
	return &fakeHandle{}, nil
}

func (f *fakeInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	f.removedHandle = handle
	return f.removeErr
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

func (f *fakeInformer) HasSyncedChecker() cache.DoneChecker {
	return nil
}

// TestFilteredInformer_RemoveEventHandler verifies that RemoveEventHandler properly delegates
// to the underlying SharedIndexInformer.
func TestFilteredInformer_RemoveEventHandler(t *testing.T) {
	fake := &fakeInformer{}
	filteredinformer := NewProviderConfigFilteredInformer(fake, "test-provider-config")

	reg, err := filteredinformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})
	if err != nil {
		t.Fatalf("AddEventHandler returned unexpected error: %v", err)
	}

	if err := filteredinformer.RemoveEventHandler(reg); err != nil {
		t.Errorf("RemoveEventHandler returned unexpected error: %v", err)
	}

	if fake.removedHandle != reg {
		t.Errorf("Expected removedHandle to be %v, got %v", reg, fake.removedHandle)
	}
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
				h.OnAdd(matchingObj, false)
			},
			expectedAddCalls: 1,
		},
		{
			desc: "OnAdd with non-matching object",
			event: func(h cache.ResourceEventHandler) {
				h.OnAdd(nonMatchingObj, false)
			},
		},
		{
			desc: "OnAdd with object without label",
			event: func(h cache.ResourceEventHandler) {
				h.OnAdd(objWithoutLabel, false)
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
		indexers: cache.Indexers{providerConfigIndexName: ProviderConfigIndexFunc},
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
		items, err := idx.ByIndex(providerConfigIndexName, providerConfigName1)
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
		items, err = idx.ByIndex(providerConfigIndexName, providerConfigName2)
		if err != nil {
			t.Fatalf("ByIndex(non-matching) returned an error: %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("ByIndex(non-matching) returned %d items, want 0, got: %v", len(items), items)
		}
	})
}
