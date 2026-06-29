package framework

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

func createProviderConfigInClient(ctx context.Context, client dynamic.Interface, pc *unstructured.Unstructured) error {
	_, err := client.Resource(testProviderConfigGVR).Create(ctx, pc, metav1.CreateOptions{})
	return err
}

func providerConfigFromClient(ctx context.Context, client dynamic.Interface, name string) (*unstructured.Unstructured, error) {
	return client.Resource(testProviderConfigGVR).Get(ctx, name, metav1.GetOptions{})
}

// mockControllerStarter is a mock implementation of ControllerStarter for testing.
type mockControllerStarter struct {
	mu                     sync.Mutex
	startCalls             int
	shouldFailStart        bool
	shouldReturnNilChannel bool
	startedControllers     map[string]chan<- struct{}
	startCounts            map[string]int
}

func newMockControllerStarter() *mockControllerStarter {
	return &mockControllerStarter{
		startedControllers: make(map[string]chan<- struct{}),
		startCounts:        make(map[string]int),
	}
}

func (m *mockControllerStarter) StartController(pc *unstructured.Unstructured) (chan<- struct{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.startCalls++
	m.startCounts[pc.GetName()]++

	if m.shouldFailStart {
		return nil, fmt.Errorf("mock start failure")
	}

	if m.shouldReturnNilChannel {
		return nil, nil
	}

	stopCh := make(chan struct{})
	m.startedControllers[pc.GetName()] = stopCh
	return stopCh, nil
}

func (m *mockControllerStarter) getStartCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startCalls
}

func createTestProviderConfig(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "cloud.gke.io/v1",
			"kind":       "ProviderConfig",
			"metadata": map[string]any{
				"name": name,
			},
			"spec": map[string]any{
				"projectID": "test-project",
			},
		},
	}
}

// hasFinalizer checks if the object has the given finalizer.
func hasFinalizer(pc *unstructured.Unstructured, key string) bool {
	for _, f := range pc.GetFinalizers() {
		if f == key {
			return true
		}
	}
	return false
}

// TestManagerStartIdempotent verifies that starting the same controller multiple times is idempotent.
func TestManagerStartIdempotent(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	manager := newManager(
		dynamicClient,
		"test-finalizer",
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")

	// Create the ProviderConfig in the fake client
	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	// First start should succeed
	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("First start failed: %v", err)
	}

	// Second start should be idempotent (no error, no additional controller started)
	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("Second start failed: %v", err)
	}

	// Should only have started once
	if mockStarter.getStartCallCount() != 1 {
		t.Errorf("Expected 1 start call, got %d", mockStarter.getStartCallCount())
	}
}

// TestManagerStartAddsFinalizerBeforeControllerStarts verifies that the finalizer
// is added before the controller starts.
func TestManagerStartAddsFinalizerBeforeControllerStarts(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	finalizerName := "test-finalizer"
	manager := newManager(
		dynamicClient,
		finalizerName,
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")

	// Create the ProviderConfig in the fake client
	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify finalizer was added
	updatedPC, err := providerConfigFromClient(ctx, dynamicClient, pc.GetName())
	if err != nil {
		t.Fatalf("Failed to get updated ProviderConfig: %v", err)
	}

	if !hasFinalizer(updatedPC, finalizerName) {
		t.Errorf("Finalizer %s was not added to ProviderConfig", finalizerName)
	}
}

// TestManagerStartFailureRollsBackFinalizer verifies that if controller startup fails,
// the finalizer is rolled back.
func TestManagerStartFailureRollsBackFinalizer(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()
	mockStarter.shouldFailStart = true

	finalizerName := "test-finalizer"
	manager := newManager(
		dynamicClient,
		finalizerName,
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")

	// Create the ProviderConfig in the fake client
	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	// Start should fail
	if err := manager.StartControllersForProviderConfig(ctx, pc); err == nil {
		t.Fatal("Expected start to fail, but it succeeded")
	}

	// Verify finalizer was rolled back
	updatedPC, err := providerConfigFromClient(ctx, dynamicClient, pc.GetName())
	if err != nil {
		t.Fatalf("Failed to get updated ProviderConfig: %v", err)
	}

	if hasFinalizer(updatedPC, finalizerName) {
		t.Errorf("Finalizer %s was not rolled back after start failure", finalizerName)
	}
}

// TestManagerStartFailureWithExistingFinalizerPreservesFinalizer verifies that a start failure
// does not remove a pre-existing finalizer that was not added by this manager instance.
func TestManagerStartFailureWithExistingFinalizerPreservesFinalizer(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()
	mockStarter.shouldFailStart = true

	finalizerName := "test-finalizer"
	manager := newManager(
		dynamicClient,
		finalizerName,
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc-existing-finalizer")
	pc.SetFinalizers([]string{finalizerName})

	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	if err := manager.StartControllersForProviderConfig(ctx, pc); err == nil {
		t.Fatal("Expected start to fail when controller start returns error")
	}

	updatedPC, err := providerConfigFromClient(ctx, dynamicClient, pc.GetName())
	if err != nil {
		t.Fatalf("Failed to get updated ProviderConfig: %v", err)
	}

	if !hasFinalizer(updatedPC, finalizerName) {
		t.Fatalf("Expected pre-existing finalizer to be preserved on failure, got %v", updatedPC.GetFinalizers())
	}
}

// TestManagerStopRemovesFinalizer verifies that stopping a controller removes the finalizer.
func TestManagerStopRemovesFinalizer(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	finalizerName := "test-finalizer"
	manager := newManager(
		dynamicClient,
		finalizerName,
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")

	// Create the ProviderConfig in the fake client
	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	// Start the controller
	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify finalizer exists
	updatedPC, err := providerConfigFromClient(ctx, dynamicClient, pc.GetName())
	if err != nil {
		t.Fatalf("Failed to get updated ProviderConfig: %v", err)
	}
	if !hasFinalizer(updatedPC, finalizerName) {
		t.Fatal("Finalizer was not added")
	}

	// Stop the controller
	if err := manager.StopControllersForProviderConfig(ctx, updatedPC); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%s) failed: %v", updatedPC.GetName(), err)
	}

	// Verify finalizer was removed
	finalPC, err := providerConfigFromClient(ctx, dynamicClient, pc.GetName())
	if err != nil {
		t.Fatalf("Failed to get final ProviderConfig: %v", err)
	}

	if hasFinalizer(finalPC, finalizerName) {
		t.Errorf("Finalizer %s was not removed after stop", finalizerName)
	}
}

// TestManagerStopIdempotent verifies that stopping a non-existent controller is safe.
func TestManagerStopIdempotent(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	manager := newManager(
		dynamicClient,
		"test-finalizer",
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")

	// Create the ProviderConfig in the fake client
	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	// Stop without start should not panic
	if err := manager.StopControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%s) failed: %v", pc.GetName(), err)
	}

	// Double stop should also be safe
	if err := manager.StopControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%s) failed: %v", pc.GetName(), err)
	}
}

// TestManagerStopRemovesFinalizerWhenNoControllerExists verifies that
// StopControllersForProviderConfig removes the finalizer even when no controller
// mapping exists (e.g., after process restart or if controller was never started).
// This ensures ProviderConfig deletion can proceed instead of stalling indefinitely.
func TestManagerStopRemovesFinalizerWhenNoControllerExists(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	finalizerName := "test-finalizer"
	manager := newManager(
		dynamicClient,
		finalizerName,
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")
	pc.SetFinalizers([]string{finalizerName})

	// Create the ProviderConfig in the fake client
	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	// Verify finalizer exists
	pcBefore, err := providerConfigFromClient(ctx, dynamicClient, pc.GetName())
	if err != nil {
		t.Fatalf("Failed to get ProviderConfig: %v", err)
	}

	if !hasFinalizer(pcBefore, finalizerName) {
		t.Fatal("Finalizer was not present on initial ProviderConfig")
	}

	// Call Stop WITHOUT ever calling Start.
	// This means no controller mapping exists in the manager.
	// The manager should still remove the finalizer regardless.
	if err := manager.StopControllersForProviderConfig(ctx, pcBefore); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%s) failed: %v", pcBefore.GetName(), err)
	}

	// Verify finalizer was removed even though no controller existed
	pcAfter, err := providerConfigFromClient(ctx, dynamicClient, pc.GetName())
	if err != nil {
		t.Fatalf("Failed to get ProviderConfig after stop: %v", err)
	}

	if hasFinalizer(pcAfter, finalizerName) {
		t.Errorf("Finalizer %s was NOT removed when no controller existed - THIS IS THE BUG", finalizerName)
	}
}

// TestManagerMultipleProviderConfigs verifies that multiple ProviderConfigs can be managed independently.
func TestManagerMultipleProviderConfigs(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	manager := newManager(
		dynamicClient,
		"test-finalizer",
		mockStarter,
	)

	pc1 := createTestProviderConfig("pc-1")
	pc2 := createTestProviderConfig("pc-2")
	pc3 := createTestProviderConfig("pc-3")

	// Create all ProviderConfigs
	for _, pc := range []*unstructured.Unstructured{pc1, pc2, pc3} {
		if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
			t.Fatalf("Failed to create ProviderConfig %s: %v", pc.GetName(), err)
		}
	}

	// Start all controllers
	var testCases = []struct {
		pc *unstructured.Unstructured
	}{
		{pc: pc1},
		{pc: pc2},
		{pc: pc3},
	}

	for _, tc := range testCases {
		if err := manager.StartControllersForProviderConfig(ctx, tc.pc); err != nil {
			t.Fatalf("Failed to start controller for %s: %v", tc.pc.GetName(), err)
		}
	}

	// Verify all controllers started
	if mockStarter.getStartCallCount() != 3 {
		t.Errorf("Expected 3 start calls, got %d", mockStarter.getStartCallCount())
	}

	// Stop one controller
	if err := manager.StopControllersForProviderConfig(ctx, pc2); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%s) failed: %v", pc2.GetName(), err)
	}

	// Start pc2 again
	if err := manager.StartControllersForProviderConfig(ctx, pc2); err != nil {
		t.Fatalf("Failed to restart controller for pc-2: %v", err)
	}

	// Should have 4 total starts now
	if mockStarter.getStartCallCount() != 4 {
		t.Errorf("Expected 4 start calls after restart, got %d", mockStarter.getStartCallCount())
	}
}

// TestManagerStartRetryWhenControllerEntryExistsButNotStarted verifies that if a controller entry exists
// but the controller is not running (stopCh is nil), StartControllersForProviderConfig will attempt to start it.
func TestManagerStartRetryWhenControllerEntryExistsButNotStarted(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	manager := newManager(
		dynamicClient,
		"test-finalizer",
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")

	// Create the ProviderConfig in the fake client
	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	// Manually inject an entry into the controller map to simulate "existed=true, stopCh=nil"
	manager.controllers.GetOrCreate(pc.GetName())

	// Attempt start.
	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Should have started
	if mockStarter.getStartCallCount() != 1 {
		t.Errorf("Expected 1 start call, got %d", mockStarter.getStartCallCount())
	}
}

// TestManagerStartFailureRemovesControllerMapEntry verifies that if controller startup fails
// for a newly created controller entry, the entry is removed from the map.
// This prevents "zombie" entries that block future start attempts.
func TestManagerStartFailureRemovesControllerMapEntry(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()
	mockStarter.shouldFailStart = true

	manager := newManager(
		dynamicClient,
		"test-finalizer",
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")

	// Create the ProviderConfig in the fake client
	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	// Start should fail
	if err := manager.StartControllersForProviderConfig(ctx, pc); err == nil {
		t.Fatal("Expected start to fail, but it succeeded")
	}

	// Verify controller map entry was removed
	if _, exists := manager.controllers.Get(pc.GetName()); exists {
		t.Errorf("Controller map entry should have been removed after start failure")
	}
}

// TestManagerStartReturnsErrorOnNilChannel verifies that StartControllersForProviderConfig
// returns an error if the controller starter returns a nil channel and nil error.
func TestManagerStartReturnsErrorOnNilChannel(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()
	mockStarter.shouldReturnNilChannel = true

	manager := newManager(
		dynamicClient,
		"test-finalizer",
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")

	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	if err := manager.StartControllersForProviderConfig(ctx, pc); err == nil {
		t.Fatal("Expected StartControllersForProviderConfig to fail when StartController returns nil channel, but it succeeded")
	}
}
