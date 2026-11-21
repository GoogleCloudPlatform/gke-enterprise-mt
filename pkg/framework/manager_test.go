package framework

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
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

func TestManagerForceCleanup(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	manager := newManager(
		dynamicClient,
		"test-finalizer",
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")
	if err := unstructured.SetNestedField(pc.Object, "test-tenant-uid", "spec", "principalInfo", "id"); err != nil {
		t.Fatalf("Failed to set tenant UID: %v", err)
	}

	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	// Start the controller
	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop the controller -> should start timer and add to deleting map
	if err := manager.StopControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	manager.deletingMu.Lock()
	state, exists := manager.deleting["test-pc"]
	manager.deletingMu.Unlock()

	if !exists {
		t.Fatal("Expected tenant to be in deleting map")
	}
	if state.tenantUID != "test-tenant-uid" {
		t.Errorf("Expected tenantUID to be test-tenant-uid, got %s", state.tenantUID)
	}
	if state.cancel == nil {
		t.Error("Expected cancel function to be set")
	}

	// Call ForceCleanupTenant
	manager.ForceCleanupTenant("test-pc")

	manager.deletingMu.Lock()
	_, exists = manager.deleting["test-pc"]
	manager.deletingMu.Unlock()

	if exists {
		t.Error("Expected tenant to be removed from deleting map after ForceCleanupTenant")
	}
}

func TestManagerStartClearsDeletingMap(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	manager := newManager(
		dynamicClient,
		"test-finalizer",
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")
	if err := unstructured.SetNestedField(pc.Object, "test-tenant-uid", "spec", "principalInfo", "id"); err != nil {
		t.Fatalf("Failed to set tenant UID: %v", err)
	}

	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	// Start the controller
	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop the controller -> should start timer and add to deleting map
	if err := manager.StopControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	manager.deletingMu.Lock()
	_, exists := manager.deleting["test-pc"]
	manager.deletingMu.Unlock()

	if !exists {
		t.Fatal("Expected tenant to be in deleting map")
	}

	// Start the controller again (simulating recreation/revival)
	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	manager.deletingMu.Lock()
	_, exists = manager.deleting["test-pc"]
	manager.deletingMu.Unlock()

	if exists {
		t.Error("Expected tenant to be removed from deleting map after starting controllers again")
	}
}

func TestManagerForceCleanupFallback(t *testing.T) {
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	manager := newManager(
		dynamicClient,
		"test-finalizer",
		mockStarter,
	)

	// Call ForceCleanupTenant for a tenant not in the deleting map.
	// It should fall back to using the key as tenantUID.
	manager.ForceCleanupTenant("test-pc-fallback")

	manager.deletingMu.Lock()
	_, exists := manager.deleting["test-pc-fallback"]
	manager.deletingMu.Unlock()

	if exists {
		t.Error("Expected tenant to not be in deleting map")
	}
}

type failNextUpdateClient struct {
	dynamic.Interface
	failNextUpdate bool
	updateCalls    int
}

type failNextUpdateResource struct {
	dynamic.NamespaceableResourceInterface
	client *failNextUpdateClient
}

func (c *failNextUpdateClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &failNextUpdateResource{
		NamespaceableResourceInterface: c.Interface.Resource(resource),
		client:                         c,
	}
}

func (r *failNextUpdateResource) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	r.client.updateCalls++
	if r.client.failNextUpdate {
		r.client.failNextUpdate = false
		return nil, fmt.Errorf("simulated transient update conflict")
	}
	return r.NamespaceableResourceInterface.Update(ctx, obj, options, subresources...)
}

func TestManagerStartUpdateFailureDoesNotMutateCachedProviderConfig(t *testing.T) {
	ctx := t.Context()
	baseClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	client := &failNextUpdateClient{
		Interface:      baseClient,
		failNextUpdate: true,
	}
	mockStarter := newMockControllerStarter()

	finalizerName := "test-finalizer"
	manager := newManager(client, finalizerName, mockStarter)

	pc := createTestProviderConfig("test-pc")
	if err := createProviderConfigInClient(ctx, baseClient, pc); err != nil {
		t.Fatalf("createProviderConfigInClient(%q) failed: %v", pc.GetName(), err)
	}

	// First call: Update fails with a transient error.
	if err := manager.StartControllersForProviderConfig(ctx, pc); err == nil { // if NO error
		t.Fatalf("StartControllersForProviderConfig(%q) = nil, want error when Update fails", pc.GetName())
	}
	if got := hasFinalizer(pc, finalizerName); got {
		t.Errorf("hasFinalizer(%q, %q) = %v, want false after failed Update", pc.GetName(), finalizerName, got)
	}
	if got, want := mockStarter.getStartCallCount(), 0; got != want {
		t.Errorf("mockStarter.getStartCallCount() = %d, want %d after failed Update", got, want)
	}

	// Second call (workqueue retry) with the exact same cached pc pointer:
	// Update must be attempted again and persist the finalizer in the client.
	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StartControllersForProviderConfig(%q) retry failed: %v", pc.GetName(), err)
	}
	if got := hasFinalizer(pc, finalizerName); got {
		t.Errorf("hasFinalizer(%q, %q) = %v, want false (cached object must not be mutated in-place)", pc.GetName(), finalizerName, got)
	}
	if got, want := client.updateCalls, 2; got != want {
		t.Errorf("client.updateCalls = %d, want %d", got, want)
	}
	if got, want := mockStarter.getStartCallCount(), 1; got != want {
		t.Errorf("mockStarter.getStartCallCount() = %d, want %d", got, want)
	}

	storedPC, err := providerConfigFromClient(ctx, baseClient, pc.GetName())
	if err != nil {
		t.Fatalf("providerConfigFromClient(%q) failed: %v", pc.GetName(), err)
	}
	if got := hasFinalizer(storedPC, finalizerName); !got {
		t.Errorf("hasFinalizer(%q, %q) = %v, want true", storedPC.GetName(), finalizerName, got)
	}
}

func newConflictError(name string) error {
	return apierrors.NewConflict(
		schema.GroupResource{Group: providerConfigGVR.Group, Resource: providerConfigGVR.Resource},
		name,
		fmt.Errorf("optimistic locking failure"),
	)
}

func TestManagerStartRetriesOnConflictAndPreservesConcurrentFinalizers(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	finalizerName := "test-finalizer"
	manager := newManager(fakeClient, finalizerName, mockStarter)

	pc := createTestProviderConfig("test-pc")
	if err := createProviderConfigInClient(ctx, fakeClient, pc); err != nil {
		t.Fatalf("createProviderConfigInClient(%q) failed: %v", pc.GetName(), err)
	}

	concurrentFinalizers := []string{"other-controller-1", "other-controller-2"}
	var conflictAttempts int
	fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if conflictAttempts < len(concurrentFinalizers) {
			concurrentFinalizer := concurrentFinalizers[conflictAttempts]
			conflictAttempts++

			tracker := fakeClient.Tracker()
			obj, err := tracker.Get(providerConfigGVR, "", pc.GetName())
			if err != nil {
				return true, nil, err
			}
			latest := obj.(*unstructured.Unstructured).DeepCopy()
			latest.SetFinalizers(append(latest.GetFinalizers(), concurrentFinalizer))
			if err := tracker.Update(providerConfigGVR, latest, ""); err != nil {
				return true, nil, err
			}
			return true, nil, newConflictError(pc.GetName())
		}
		return false, nil, nil
	})

	// Pass stale pc without any finalizers.
	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StartControllersForProviderConfig(%q) failed: %v", pc.GetName(), err)
	}
	if got, want := conflictAttempts, len(concurrentFinalizers); got != want {
		t.Errorf("conflictAttempts = %d, want %d", got, want)
	}
	if got, want := mockStarter.getStartCallCount(), 1; got != want {
		t.Errorf("mockStarter.getStartCallCount() = %d, want %d", got, want)
	}
	if hasFinalizer(pc, finalizerName) {
		t.Errorf("cached ProviderConfig should not be mutated in-place, got finalizers %v", pc.GetFinalizers())
	}

	storedPC, err := providerConfigFromClient(ctx, fakeClient, pc.GetName())
	if err != nil {
		t.Fatalf("providerConfigFromClient(%q) failed: %v", pc.GetName(), err)
	}
	wantFinalizers := []string{"other-controller-1", "other-controller-2", finalizerName}
	if diff := cmp.Diff(wantFinalizers, storedPC.GetFinalizers()); diff != "" {
		t.Errorf("stored finalizers mismatch (-want +got):\n%s", diff)
	}
}

func TestManagerStartFailsAfterConflictRetriesExhausted(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	manager := newManager(fakeClient, "test-finalizer", mockStarter)

	pc := createTestProviderConfig("test-pc")
	if err := createProviderConfigInClient(ctx, fakeClient, pc); err != nil {
		t.Fatalf("createProviderConfigInClient(%q) failed: %v", pc.GetName(), err)
	}

	var updateAttempts int
	fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		return true, nil, newConflictError(pc.GetName())
	})

	err := manager.StartControllersForProviderConfig(ctx, pc)
	if !apierrors.IsConflict(err) {
		t.Fatalf("StartControllersForProviderConfig(%q) error = %v, want conflict error", pc.GetName(), err)
	}
	if updateAttempts <= 1 {
		t.Errorf("updateAttempts = %d, want > 1 after conflict retries", updateAttempts)
	}
	if got := mockStarter.getStartCallCount(); got != 0 {
		t.Errorf("mockStarter.getStartCallCount() = %d, want 0", got)
	}
	if _, exists := manager.controllers.Get(pc.GetName()); exists {
		t.Errorf("controller map entry should be removed after finalizer failure")
	}
}

func TestManagerStartFailureRollbackRetriesOnConflictAndPreservesConcurrentFinalizers(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()
	mockStarter.shouldFailStart = true

	finalizerName := "test-finalizer"
	manager := newManager(fakeClient, finalizerName, mockStarter)

	pc := createTestProviderConfig("test-pc")
	pc.SetFinalizers([]string{"other-controller-1"})
	if err := createProviderConfigInClient(ctx, fakeClient, pc); err != nil {
		t.Fatalf("createProviderConfigInClient(%q) failed: %v", pc.GetName(), err)
	}

	// Pass a local copy without finalizerName so Start adds it, then fails and rolls back.
	localPC := createTestProviderConfig("test-pc")

	var updateCount int
	fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		updateCount++
		// Let the initial add succeed (updateCount == 1); inject 409 on the first rollback attempt (updateCount == 2).
		if updateCount == 2 {
			tracker := fakeClient.Tracker()
			obj, err := tracker.Get(providerConfigGVR, "", pc.GetName())
			if err != nil {
				return true, nil, err
			}
			latest := obj.(*unstructured.Unstructured).DeepCopy()
			latest.SetFinalizers(append(latest.GetFinalizers(), "other-controller-2"))
			if err := tracker.Update(providerConfigGVR, latest, ""); err != nil {
				return true, nil, err
			}
			return true, nil, newConflictError(pc.GetName())
		}
		return false, nil, nil
	})

	if err := manager.StartControllersForProviderConfig(ctx, localPC); err == nil {
		t.Fatal("StartControllersForProviderConfig() = nil, want start error")
	}
	if got, want := updateCount, 3; got != want {
		t.Errorf("updateCount = %d, want %d (1 add + 1 conflicted rollback + 1 retried rollback)", got, want)
	}

	storedPC, err := providerConfigFromClient(ctx, fakeClient, pc.GetName())
	if err != nil {
		t.Fatalf("providerConfigFromClient(%q) failed: %v", pc.GetName(), err)
	}
	wantFinalizers := []string{"other-controller-1", "other-controller-2"}
	if diff := cmp.Diff(wantFinalizers, storedPC.GetFinalizers()); diff != "" {
		t.Errorf("stored finalizers after rollback mismatch (-want +got):\n%s", diff)
	}
}

func TestManagerStopRetriesOnConflictAndPreservesConcurrentFinalizers(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	finalizerName := "test-finalizer"
	manager := newManager(fakeClient, finalizerName, mockStarter)

	pc := createTestProviderConfig("test-pc")
	pc.SetFinalizers([]string{"other-controller-1", finalizerName, "other-controller-2"})
	if err := createProviderConfigInClient(ctx, fakeClient, pc); err != nil {
		t.Fatalf("createProviderConfigInClient(%q) failed: %v", pc.GetName(), err)
	}

	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StartControllersForProviderConfig(%q) failed: %v", pc.GetName(), err)
	}

	conflictInjected := false
	fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if conflictInjected {
			return false, nil, nil
		}
		conflictInjected = true

		// Simulate another controller concurrently removing other-controller-1 and adding other-controller-3.
		tracker := fakeClient.Tracker()
		obj, err := tracker.Get(providerConfigGVR, "", pc.GetName())
		if err != nil {
			return true, nil, err
		}
		latest := obj.(*unstructured.Unstructured).DeepCopy()
		latest.SetFinalizers([]string{finalizerName, "other-controller-2", "other-controller-3"})
		if err := tracker.Update(providerConfigGVR, latest, ""); err != nil {
			return true, nil, err
		}
		return true, nil, newConflictError(pc.GetName())
	})

	if err := manager.StopControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%q) failed: %v", pc.GetName(), err)
	}
	if !conflictInjected {
		t.Fatal("expected conflict reactor to trigger")
	}

	storedPC, err := providerConfigFromClient(ctx, fakeClient, pc.GetName())
	if err != nil {
		t.Fatalf("providerConfigFromClient(%q) failed: %v", pc.GetName(), err)
	}
	wantFinalizers := []string{"other-controller-2", "other-controller-3"}
	if diff := cmp.Diff(wantFinalizers, storedPC.GetFinalizers()); diff != "" {
		t.Errorf("stored finalizers after stop mismatch (-want +got):\n%s", diff)
	}
}

func TestManagerStopHandlesNotFoundDuringUpdate(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	finalizerName := "test-finalizer"
	manager := newManager(fakeClient, finalizerName, mockStarter)

	pc := createTestProviderConfig("test-pc")
	pc.SetFinalizers([]string{finalizerName})
	if err := createProviderConfigInClient(ctx, fakeClient, pc); err != nil {
		t.Fatalf("createProviderConfigInClient(%q) failed: %v", pc.GetName(), err)
	}

	fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		_ = fakeClient.Tracker().Delete(providerConfigGVR, "", pc.GetName())
		return true, nil, apierrors.NewNotFound(
			schema.GroupResource{Group: providerConfigGVR.Group, Resource: providerConfigGVR.Resource},
			pc.GetName(),
		)
	})

	if err := manager.StopControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%q) returned unexpected error when deleted concurrently: %v", pc.GetName(), err)
	}
}

func TestManagerConcurrentControllersFinalizerRace(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)

	// Enforce ResourceVersion optimistic locking on the fake client just like kube-apiserver.
	var trackerMu sync.Mutex
	var nextRV int64 = 1
	fakeClient.PrependReactor("create", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		trackerMu.Lock()
		defer trackerMu.Unlock()
		createAction := action.(k8stesting.CreateAction)
		obj := createAction.GetObject().(*unstructured.Unstructured).DeepCopy()
		obj.SetResourceVersion(strconv.FormatInt(nextRV, 10))
		nextRV++
		if err := fakeClient.Tracker().Add(obj); err != nil {
			return true, nil, err
		}
		return true, obj.DeepCopy(), nil
	})
	fakeClient.PrependReactor("get", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		trackerMu.Lock()
		defer trackerMu.Unlock()
		getAction := action.(k8stesting.GetAction)
		obj, err := fakeClient.Tracker().Get(providerConfigGVR, "", getAction.GetName())
		if err != nil {
			return true, nil, err
		}
		return true, obj.(*unstructured.Unstructured).DeepCopy(), nil
	})
	fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		trackerMu.Lock()
		defer trackerMu.Unlock()
		updateAction := action.(k8stesting.UpdateAction)
		incoming := updateAction.GetObject().(*unstructured.Unstructured).DeepCopy()
		curObj, err := fakeClient.Tracker().Get(providerConfigGVR, "", incoming.GetName())
		if err != nil {
			return true, nil, err
		}
		current := curObj.(*unstructured.Unstructured)
		if incoming.GetResourceVersion() != current.GetResourceVersion() {
			return true, nil, newConflictError(incoming.GetName())
		}
		incoming.SetResourceVersion(strconv.FormatInt(nextRV, 10))
		nextRV++
		if err := fakeClient.Tracker().Update(providerConfigGVR, incoming, ""); err != nil {
			return true, nil, err
		}
		return true, incoming.DeepCopy(), nil
	})

	initialPC := createTestProviderConfig("shared-pc")
	createdPC, err := fakeClient.Resource(providerConfigGVR).Create(ctx, initialPC, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(%q) failed: %v", initialPC.GetName(), err)
	}

	finalizers := []string{
		"ccm.gke.io/finalizer",
		"neg.gke.io/finalizer",
		"clouddns.gke.io/finalizer",
		"gateway.gke.io/finalizer",
		"filestorecsi.gke.io/finalizer",
	}

	managers := make([]*manager, len(finalizers))
	for i, fin := range finalizers {
		managers[i] = newManager(fakeClient, fin, newMockControllerStarter())
	}

	// All 5 controllers observe the newly created ProviderConfig simultaneously.
	var startWG sync.WaitGroup
	startErrs := make([]error, len(managers))
	for i, mgr := range managers {
		startWG.Add(1)
		go func(idx int, m *manager, cachedPC *unstructured.Unstructured) {
			defer startWG.Done()
			startErrs[idx] = m.StartControllersForProviderConfig(ctx, cachedPC)
		}(i, mgr, createdPC.DeepCopy())
	}
	startWG.Wait()

	for i, err := range startErrs {
		if err != nil {
			t.Fatalf("manager %d (%s) StartControllersForProviderConfig failed: %v", i, finalizers[i], err)
		}
	}

	storedAfterStart, err := providerConfigFromClient(ctx, fakeClient, createdPC.GetName())
	if err != nil {
		t.Fatalf("providerConfigFromClient(%q) after start failed: %v", createdPC.GetName(), err)
	}
	gotStartFinalizers := slices.Clone(storedAfterStart.GetFinalizers())
	slices.Sort(gotStartFinalizers)
	wantStartFinalizers := slices.Clone(finalizers)
	slices.Sort(wantStartFinalizers)
	if diff := cmp.Diff(wantStartFinalizers, gotStartFinalizers); diff != "" {
		t.Errorf("finalizers after concurrent starts mismatch (-want +got):\n%s", diff)
	}

	// All 5 controllers observe DeletionTimestamp and stop simultaneously.
	deletingPC := storedAfterStart.DeepCopy()
	now := metav1.Now()
	deletingPC.SetDeletionTimestamp(&now)

	var stopWG sync.WaitGroup
	stopErrs := make([]error, len(managers))
	for i, mgr := range managers {
		stopWG.Add(1)
		go func(idx int, m *manager, cachedPC *unstructured.Unstructured) {
			defer stopWG.Done()
			stopErrs[idx] = m.StopControllersForProviderConfig(ctx, cachedPC)
		}(i, mgr, deletingPC.DeepCopy())
	}
	stopWG.Wait()

	for i, err := range stopErrs {
		if err != nil {
			t.Fatalf("manager %d (%s) StopControllersForProviderConfig failed: %v", i, finalizers[i], err)
		}
	}

	storedAfterStop, err := providerConfigFromClient(ctx, fakeClient, createdPC.GetName())
	if err != nil {
		t.Fatalf("providerConfigFromClient(%q) after stop failed: %v", createdPC.GetName(), err)
	}
	if got := storedAfterStop.GetFinalizers(); len(got) != 0 {
		t.Errorf("finalizers after concurrent stops = %v, want empty", got)
	}
}

// cancelAfterUpdate makes the fake client apply each ProviderConfig update and
// then cancel the context, as if the caller was canceled right after the API
// server committed the write.
func cancelAfterUpdate(fakeClient *fake.FakeDynamicClient, cancel context.CancelFunc) {
	fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		obj := action.(k8stesting.UpdateAction).GetObject().(*unstructured.Unstructured).DeepCopy()
		if err := fakeClient.Tracker().Update(providerConfigGVR, obj, ""); err != nil {
			return true, nil, err
		}
		cancel()
		return true, obj, nil
	})
}

func TestManagerStartSucceedsWhenContextCanceledAfterFinalizerUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	finalizerName := "test-finalizer"
	manager := newManager(fakeClient, finalizerName, mockStarter)

	pc := createTestProviderConfig("test-pc")
	if err := createProviderConfigInClient(ctx, fakeClient, pc); err != nil {
		t.Fatalf("createProviderConfigInClient(%q) failed: %v", pc.GetName(), err)
	}
	cancelAfterUpdate(fakeClient, cancel)

	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StartControllersForProviderConfig(%q) = %v, want nil after the finalizer update succeeded", pc.GetName(), err)
	}
	if got, want := mockStarter.getStartCallCount(), 1; got != want {
		t.Errorf("mockStarter.getStartCallCount() = %d, want %d", got, want)
	}

	storedPC, err := providerConfigFromClient(context.Background(), fakeClient, pc.GetName())
	if err != nil {
		t.Fatalf("providerConfigFromClient(%q) failed: %v", pc.GetName(), err)
	}
	if !hasFinalizer(storedPC, finalizerName) {
		t.Errorf("stored finalizers = %v, want %q present", storedPC.GetFinalizers(), finalizerName)
	}
}

func TestManagerStopSucceedsWhenContextCanceledAfterFinalizerUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)

	finalizerName := "test-finalizer"
	manager := newManager(fakeClient, finalizerName, newMockControllerStarter())

	pc := createTestProviderConfig("test-pc")
	pc.SetFinalizers([]string{"other-controller-1", finalizerName})
	if err := createProviderConfigInClient(ctx, fakeClient, pc); err != nil {
		t.Fatalf("createProviderConfigInClient(%q) failed: %v", pc.GetName(), err)
	}
	cancelAfterUpdate(fakeClient, cancel)

	if err := manager.StopControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%q) = %v, want nil after the finalizer update succeeded", pc.GetName(), err)
	}

	storedPC, err := providerConfigFromClient(context.Background(), fakeClient, pc.GetName())
	if err != nil {
		t.Fatalf("providerConfigFromClient(%q) failed: %v", pc.GetName(), err)
	}
	if diff := cmp.Diff([]string{"other-controller-1"}, storedPC.GetFinalizers()); diff != "" {
		t.Errorf("stored finalizers after stop mismatch (-want +got):\n%s", diff)
	}
}

// TestManagerStartSkipsProviderConfigAlreadyTerminatingOnServer covers a stale
// informer cache: the cached object is live and has no finalizer, but the API
// server already has a DeletionTimestamp on it.
func TestManagerStartSkipsProviderConfigAlreadyTerminatingOnServer(t *testing.T) {
	const finalizerName = "test-finalizer"
	tests := []struct {
		name             string
		serverFinalizers []string
	}{
		{
			name:             "finalizer absent on server",
			serverFinalizers: []string{"other-controller-1"},
		},
		{
			name:             "finalizer present on server",
			serverFinalizers: []string{"other-controller-1", finalizerName},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
			mockStarter := newMockControllerStarter()
			manager := newManager(fakeClient, finalizerName, mockStarter)

			serverPC := createTestProviderConfig("test-pc")
			serverPC.SetFinalizers(tc.serverFinalizers)
			now := metav1.Now()
			serverPC.SetDeletionTimestamp(&now)
			if err := createProviderConfigInClient(ctx, fakeClient, serverPC); err != nil {
				t.Fatalf("createProviderConfigInClient(%q) failed: %v", serverPC.GetName(), err)
			}

			var updateCalls int
			fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
				updateCalls++
				return false, nil, nil
			})

			cachedPC := createTestProviderConfig("test-pc")
			if err := manager.StartControllersForProviderConfig(ctx, cachedPC); err != nil {
				t.Fatalf("StartControllersForProviderConfig(%q) = %v, want nil for a terminating ProviderConfig", cachedPC.GetName(), err)
			}
			if got := mockStarter.getStartCallCount(); got != 0 {
				t.Errorf("mockStarter.getStartCallCount() = %d, want 0", got)
			}
			if updateCalls != 0 {
				t.Errorf("updateCalls = %d, want 0", updateCalls)
			}
			if _, exists := manager.controllers.Get(cachedPC.GetName()); exists {
				t.Errorf("controller map entry should be removed for a terminating ProviderConfig")
			}

			storedPC, err := providerConfigFromClient(ctx, fakeClient, cachedPC.GetName())
			if err != nil {
				t.Fatalf("providerConfigFromClient(%q) failed: %v", cachedPC.GetName(), err)
			}
			if diff := cmp.Diff(tc.serverFinalizers, storedPC.GetFinalizers()); diff != "" {
				t.Errorf("stored finalizers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestManagerStartSkipsWhenDeletionStartsDuringConflictRetry(t *testing.T) {
	ctx := t.Context()
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	finalizerName := "test-finalizer"
	manager := newManager(fakeClient, finalizerName, mockStarter)

	pc := createTestProviderConfig("test-pc")
	pc.SetFinalizers([]string{"other-controller-1"})
	if err := createProviderConfigInClient(ctx, fakeClient, pc); err != nil {
		t.Fatalf("createProviderConfigInClient(%q) failed: %v", pc.GetName(), err)
	}

	// The first update loses the race to a delete request, which sets the
	// DeletionTimestamp and returns a conflict.
	var updateCalls int
	fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		updateCalls++
		if updateCalls > 1 {
			return false, nil, nil
		}
		tracker := fakeClient.Tracker()
		obj, err := tracker.Get(providerConfigGVR, "", pc.GetName())
		if err != nil {
			return true, nil, err
		}
		latest := obj.(*unstructured.Unstructured).DeepCopy()
		now := metav1.Now()
		latest.SetDeletionTimestamp(&now)
		if err := tracker.Update(providerConfigGVR, latest, ""); err != nil {
			return true, nil, err
		}
		return true, nil, newConflictError(pc.GetName())
	})

	if err := manager.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StartControllersForProviderConfig(%q) = %v, want nil for a terminating ProviderConfig", pc.GetName(), err)
	}
	if got, want := updateCalls, 1; got != want {
		t.Errorf("updateCalls = %d, want %d (no update after deletion started)", got, want)
	}
	if got := mockStarter.getStartCallCount(); got != 0 {
		t.Errorf("mockStarter.getStartCallCount() = %d, want 0", got)
	}
	if _, exists := manager.controllers.Get(pc.GetName()); exists {
		t.Errorf("controller map entry should be removed for a terminating ProviderConfig")
	}

	storedPC, err := providerConfigFromClient(ctx, fakeClient, pc.GetName())
	if err != nil {
		t.Fatalf("providerConfigFromClient(%q) failed: %v", pc.GetName(), err)
	}
	if diff := cmp.Diff([]string{"other-controller-1"}, storedPC.GetFinalizers()); diff != "" {
		t.Errorf("stored finalizers mismatch (-want +got):\n%s", diff)
	}
}
