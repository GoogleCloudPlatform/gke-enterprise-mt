package framework

import (
	"testing"
)

// TestControllerMapBasicOperations verifies basic Get, GetOrCreate, and Delete operations.
func TestControllerMapBasicOperations(t *testing.T) {
	cm := NewControllerMap()

	testKey := "test-key"
	nonExistentKey := "non-existent"

	// Test Get on non-existent key
	_, exists := cm.Get(testKey)
	if exists {
		t.Error("Expected controller to not exist")
	}

	// Test GetOrCreate creates new entry
	cs, existed := cm.GetOrCreate(testKey)
	if existed {
		t.Error("Expected controller to not exist before GetOrCreate")
	}
	if cs == nil {
		t.Error("Expected GetOrCreate to return a ControllerSet")
	}

	// Test Get returns the same entry
	retrievedCS, exists := cm.Get(testKey)
	if !exists {
		t.Error("Expected controller to exist after GetOrCreate")
	}
	if retrievedCS != cs {
		t.Error("Retrieved controller does not match created controller")
	}

	// Test Delete
	cm.Delete(testKey)

	// Test Get after Delete
	_, exists = cm.Get(testKey)
	if exists {
		t.Error("Expected controller to not exist after Delete")
	}

	// Test Delete on non-existent key (should not panic)
	cm.Delete(nonExistentKey)
}

// TestControllerMapGetOrCreate verifies GetOrCreate idempotency.
func TestControllerMapGetOrCreate(t *testing.T) {
	cm := NewControllerMap()

	key := "alpha"
	first, existed := cm.GetOrCreate(key)
	if existed {
		t.Fatal("Expected first GetOrCreate call to report non-existence")
	}
	if first == nil {
		t.Fatal("Expected ControllerSet instance on first GetOrCreate call")
	}

	second, existed := cm.GetOrCreate(key)
	if !existed {
		t.Fatal("Expected second GetOrCreate call to report existence")
	}
	if first != second {
		t.Fatal("Expected GetOrCreate to return the same ControllerSet instance")
	}
}
