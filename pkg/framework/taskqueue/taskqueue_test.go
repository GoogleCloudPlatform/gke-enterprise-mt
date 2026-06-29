package taskqueue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/tools/cache"
)

func TestPeriodicQueueWithMultipleWorkers(t *testing.T) {
	t.Parallel()
	// Use a sync map since multiple goroutines will write to disjoint keys in parallel.
	synced := sync.Map{}
	sync := func(_ context.Context, key string) error {
		synced.Store(key, true)
		switch key {
		case "err":
			return errors.New("injected error")
		}
		return nil
	}
	validInputObjs := []string{"a", "b", "c", "d", "e", "f", "g"}
	inputObjsWithErr := []string{"a", "b", "c", "d", "e", "f", "err", "g"}
	testCases := []struct {
		desc                string
		numWorkers          int
		expectRequeueForKey string
		inputObjs           []string
		expectNil           bool
	}{
		{"queue with 0 workers should fail", 0, "", nil, true},
		{"queue with 1 worker should work", 1, "", validInputObjs, false},
		{"queue with multiple workers should work", 5, "", validInputObjs, false},
		{"queue with multiple workers should requeue errors", 5, "err", inputObjsWithErr, false},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tq := NewPeriodicTaskQueueWithMultipleWorkers("multiple-workers", "test", tc.numWorkers, sync)
			gotNil := tq == nil
			if gotNil != tc.expectNil {
				t.Errorf("gotNilQueue - %v, expectNilQueue - %v", gotNil, tc.expectNil)
			}
			if tq == nil {
				return
			}
			// Spawn off worker routines in parallel.
			tq.Run()

			for _, obj := range tc.inputObjs {
				tq.Enqueue(cache.ExplicitKey(obj))
			}

			for tq.Len() > 0 {
				time.Sleep(1 * time.Second)
			}

			if tc.expectRequeueForKey != "" {
				if tq.queue.NumRequeues(tc.expectRequeueForKey) == 0 {
					t.Errorf("Got 0 requeues for %q, expected non-zero requeue on error", tc.expectRequeueForKey)
				}
				if tq.NumRequeues(cache.ExplicitKey(tc.expectRequeueForKey)) == 0 {
					t.Errorf("NumRequeues(%q) returned 0, expected non-zero requeue on error", tc.expectRequeueForKey)
				}
			}
			tq.Shutdown()

			// Enqueue after Shutdown isn't going to be synced.
			tq.Enqueue(cache.ExplicitKey("more"))

			syncedLen := 0
			synced.Range(func(_, _ any) bool {
				syncedLen++
				return true
			})

			if syncedLen != len(tc.inputObjs) {
				t.Errorf("Synced %d keys, but %d input keys were provided", syncedLen, len(tc.inputObjs))
			}
			for _, key := range tc.inputObjs {
				if _, ok := synced.Load(key); !ok {
					t.Errorf("Did not sync input key - %s", key)
				}
			}
		})
	}
}

// TestPeriodicQueueForgetResetsRequeues verifies that the task queue calls Forget() on a key after
// a successful sync, which should reset the number of requeues for that key. It simulates a flaky
// key that fails twice before succeeding.
func TestPeriodicQueueForgetResetsRequeues(t *testing.T) {
	t.Parallel()
	var lock sync.Mutex
	failCount := 0
	key := "flaky-key"
	checkerKey := "checker-key"

	// We need a channel to report the result from the checker.
	// Using a buffered channel to avoid blocking.
	resultCh := make(chan int, 1)

	// We need tq variable to be available in sync, but it's not created yet.
	var tq *PeriodicTaskQueueWithMultipleWorkers

	syncFn := func(_ context.Context, k string) error {
		if k == key {
			lock.Lock()
			defer lock.Unlock()
			if failCount < 2 {
				failCount++
				return errors.New("injected error")
			}
			// On success (3rd attempt), enqueue the checker.
			// We must ensure tq is set.
			if tq != nil {
				tq.Enqueue(cache.ExplicitKey(checkerKey))
			}
			return nil
		}
		if k == checkerKey {
			// Check re-queues for the flaky key.
			if tq != nil {
				resultCh <- tq.NumRequeues(cache.ExplicitKey(key))
			}
		}
		return nil
	}

	tq = NewPeriodicTaskQueueWithMultipleWorkers("forget-queue", "test", 1, syncFn)
	if tq == nil {
		t.Fatal("Failed to create task queue")
	}
	tq.Run()
	defer tq.Shutdown()

	tq.Enqueue(cache.ExplicitKey(key))

	// Wait for result with timeout
	select {
	case numRequeues := <-resultCh:
		if numRequeues != 0 {
			t.Errorf("Expected 0 requeues after success, got %d", numRequeues)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for checker to run")
	}
}

// TestShutdownWaitsForActiveWorkers verifies that Shutdown blocks until all currently active
// workers have finished processing their current items.
func TestShutdownWaitsForActiveWorkers(t *testing.T) {
	t.Parallel()
	// blockCh is used to block the worker execution until we are ready to let it finish.
	blockCh := make(chan struct{})
	// syncCalled is used to signal that the worker has picked up the item and started execution.
	syncCalled := make(chan struct{})

	syncFn := func(_ context.Context, _ string) error {
		close(syncCalled)
		<-blockCh
		return nil
	}

	tq := NewPeriodicTaskQueueWithMultipleWorkers("shutdown-test", "test", 1, syncFn)
	if tq == nil {
		t.Fatal("Failed to create task queue")
	}
	tq.Run()

	tq.Enqueue(cache.ExplicitKey("item"))

	// Wait for the worker to pick up the item
	<-syncCalled

	shutdownDone := make(chan struct{})
	go func() {
		tq.Shutdown()
		close(shutdownDone)
	}()

	// Shutdown should be blocked waiting for worker
	select {
	case <-shutdownDone:
		t.Fatal("Shutdown returned while worker was still running")
	case <-time.After(100 * time.Millisecond):
		// Expected behavior
	}

	// Unblock worker
	close(blockCh)

	// Shutdown should now complete
	select {
	case <-shutdownDone:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Shutdown did not return after worker finished")
	}
}

// TestQueueCreationWithEmptyName verifies that creating a queue with an empty name
// works correctly and falls back to NewRateLimitingQueue (unnamed).
func TestQueueCreationWithEmptyName(t *testing.T) {
	t.Parallel()
	doneCh := make(chan struct{})
	syncFn := func(_ context.Context, key string) error {
		if key != "test-item" {
			t.Errorf("Expected key 'test-item', got %q", key)
		}
		close(doneCh)
		return nil
	}

	// Pass empty string for name
	tq := NewPeriodicTaskQueueWithMultipleWorkers("", "test-resource", 1, syncFn)
	if tq == nil {
		t.Fatal("Failed to create task queue with empty name")
	}

	tq.Run()
	defer tq.Shutdown()

	tq.Enqueue(cache.ExplicitKey("test-item"))

	select {
	case <-doneCh:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for item to be processed")
	}
}

// TestEnqueueError verifies that Enqueue handles errors from keyFunc correctly by not adding
// the item to the queue.
func TestEnqueueError(t *testing.T) {
	t.Parallel()
	syncFn := func(_ context.Context, _ string) error { return nil }
	tq := NewPeriodicTaskQueueWithMultipleWorkers("error-queue", "test", 1, syncFn)

	// Override keyFunc to always return error
	tq.keyFunc = func(_ any) (string, error) {
		return "", errors.New("injected error")
	}

	tq.Enqueue("item")

	if tq.Len() != 0 {
		t.Errorf("Expected queue length 0 after Enqueue error, got %d", tq.Len())
	}
}
