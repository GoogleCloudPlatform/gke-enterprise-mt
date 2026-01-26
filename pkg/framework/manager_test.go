package framework

import (
	"context"
	"fmt"
	"sync"
	"testing"

	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"
	crv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/providerconfigcr" // implicitly needed if used or for consistency
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	// Register the ProviderConfig types with the scheme
	providerconfigv1.AddToScheme(scheme.Scheme)
}

func createProviderConfigInClient(ctx context.Context, client dynamic.Interface, pc *providerconfigv1.ProviderConfig) error {
	_, err := client.Resource(crv1.ProviderConfigGVR).Create(ctx, toUnstructured(pc), metav1.CreateOptions{})
	return err
}

func providerConfigFromClient(ctx context.Context, client dynamic.Interface, name string) (*providerconfigv1.ProviderConfig, error) {
	u, err := client.Resource(crv1.ProviderConfigGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return crv1.NewProviderConfig(u)
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

func (m *mockControllerStarter) StartController(pc *providerconfigv1.ProviderConfig) (chan<- struct{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.startCalls++
	m.startCounts[pc.Name]++

	if m.shouldFailStart {
		return nil, fmt.Errorf("mock start failure")
	}

	if m.shouldReturnNilChannel {
		return nil, nil
	}

	stopCh := make(chan struct{})
	m.startedControllers[pc.Name] = stopCh
	return stopCh, nil
}

func (m *mockControllerStarter) getStartCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startCalls
}

func createTestProviderConfig(name string) *providerconfigv1.ProviderConfig {
	return &providerconfigv1.ProviderConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ProviderConfig",
			APIVersion: "cloud.gke.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: providerconfigv1.ProviderConfigSpec{
			ProjectID: "test-project",
		},
	}
}

// hasFinalizer checks if the object has the given finalizer.
func hasFinalizer(m metav1.Object, key string) bool {
	for _, f := range m.GetFinalizers() {
		if f == key {
			return true
		}
	}
	return false
}

// TestManagerStartIdempotent verifies that starting the same controller multiple times is idempotent.
func TestManagerStartIdempotent(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
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
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
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
	updatedPC, err := providerConfigFromClient(ctx, dynamicClient, pc.Name)
	if err != nil {
		t.Fatalf("Failed to get updated ProviderConfig: %v", err)
	}

	if !hasFinalizer(&updatedPC.ObjectMeta, finalizerName) {
		t.Errorf("Finalizer %s was not added to ProviderConfig", finalizerName)
	}
}

// TestManagerStartFailureRollsBackFinalizer verifies that if controller startup fails,
// the finalizer is rolled back.
func TestManagerStartFailureRollsBackFinalizer(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
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
	updatedPC, err := providerConfigFromClient(ctx, dynamicClient, pc.Name)
	if err != nil {
		t.Fatalf("Failed to get updated ProviderConfig: %v", err)
	}

	if hasFinalizer(&updatedPC.ObjectMeta, finalizerName) {
		t.Errorf("Finalizer %s was not rolled back after start failure", finalizerName)
	}
}

// TestManagerStartFailureWithExistingFinalizerPreservesFinalizer verifies that a start failure
// does not remove a pre-existing finalizer that was not added by this manager instance.
func TestManagerStartFailureWithExistingFinalizerPreservesFinalizer(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
	mockStarter := newMockControllerStarter()
	mockStarter.shouldFailStart = true

	finalizerName := "test-finalizer"
	manager := newManager(
		dynamicClient,
		finalizerName,
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc-existing-finalizer")
	pc.Finalizers = []string{finalizerName}

	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	if err := manager.StartControllersForProviderConfig(ctx, pc); err == nil {
		t.Fatal("Expected start to fail when controller start returns error")
	}

	updatedPC, err := providerConfigFromClient(ctx, dynamicClient, pc.Name)
	if err != nil {
		t.Fatalf("Failed to get updated ProviderConfig: %v", err)
	}

	if !hasFinalizer(&updatedPC.ObjectMeta, finalizerName) {
		t.Fatalf("Expected pre-existing finalizer to be preserved on failure, got %v", updatedPC.Finalizers)
	}
}

// TestManagerStopRemovesFinalizer verifies that stopping a controller removes the finalizer.
func TestManagerStopRemovesFinalizer(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
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
	updatedPC, err := providerConfigFromClient(ctx, dynamicClient, pc.Name)
	if err != nil {
		t.Fatalf("Failed to get updated ProviderConfig: %v", err)
	}
	if !hasFinalizer(&updatedPC.ObjectMeta, finalizerName) {
		t.Fatal("Finalizer was not added")
	}

	// Stop the controller
	if err := manager.StopControllersForProviderConfig(ctx, updatedPC); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%s) failed: %v", updatedPC.Name, err)
	}

	// Verify finalizer was removed
	finalPC, err := providerConfigFromClient(ctx, dynamicClient, pc.Name)
	if err != nil {
		t.Fatalf("Failed to get final ProviderConfig: %v", err)
	}

	if hasFinalizer(&finalPC.ObjectMeta, finalizerName) {
		t.Errorf("Finalizer %s was not removed after stop", finalizerName)
	}
}

// TestManagerStopIdempotent verifies that stopping a non-existent controller is safe.
func TestManagerStopIdempotent(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
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
		t.Fatalf("StopControllersForProviderConfig(%s) failed: %v", pc.Name, err)
	}

	// Double stop should also be safe
	if err := manager.StopControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%s) failed: %v", pc.Name, err)
	}
}

// TestManagerStopRemovesFinalizerWhenNoControllerExists verifies that
// StopControllersForProviderConfig removes the finalizer even when no controller
// mapping exists (e.g., after process restart or if controller was never started).
// This ensures ProviderConfig deletion can proceed instead of stalling indefinitely.
func TestManagerStopRemovesFinalizerWhenNoControllerExists(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
	mockStarter := newMockControllerStarter()

	finalizerName := "test-finalizer"
	manager := newManager(
		dynamicClient,
		finalizerName,
		mockStarter,
	)

	pc := createTestProviderConfig("test-pc")
	pc.Finalizers = []string{finalizerName}

	// Create the ProviderConfig in the fake client
	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	// Verify finalizer exists
	pcBefore, err := providerConfigFromClient(ctx, dynamicClient, pc.Name)
	if err != nil {
		t.Fatalf("Failed to get ProviderConfig: %v", err)
	}

	if !hasFinalizer(&pcBefore.ObjectMeta, finalizerName) {
		t.Fatal("Finalizer was not present on initial ProviderConfig")
	}

	// Call Stop WITHOUT ever calling Start.
	// This means no controller mapping exists in the manager.
	// The manager should still remove the finalizer regardless.
	if err := manager.StopControllersForProviderConfig(ctx, pcBefore); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%s) failed: %v", pcBefore.Name, err)
	}

	// Verify finalizer was removed even though no controller existed
	pcAfter, err := providerConfigFromClient(ctx, dynamicClient, pc.Name)
	if err != nil {
		t.Fatalf("Failed to get ProviderConfig after stop: %v", err)
	}

	if hasFinalizer(&pcAfter.ObjectMeta, finalizerName) {
		t.Errorf("Finalizer %s was NOT removed when no controller existed - THIS IS THE BUG", finalizerName)
	}
}

// TestManagerMultipleProviderConfigs verifies that multiple ProviderConfigs can be managed independently.
func TestManagerMultipleProviderConfigs(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
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
	for _, pc := range []*providerconfigv1.ProviderConfig{pc1, pc2, pc3} {
		if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
			t.Fatalf("Failed to create ProviderConfig %s: %v", pc.Name, err)
		}
	}

	// Start all controllers
	var testCases = []struct {
		pc *providerconfigv1.ProviderConfig
	}{
		{pc: pc1},
		{pc: pc2},
		{pc: pc3},
	}

	for _, tc := range testCases {
		if err := manager.StartControllersForProviderConfig(ctx, tc.pc); err != nil {
			t.Fatalf("Failed to start controller for %s: %v", tc.pc.Name, err)
		}
	}

	// Verify all controllers started
	if mockStarter.getStartCallCount() != 3 {
		t.Errorf("Expected 3 start calls, got %d", mockStarter.getStartCallCount())
	}

	// Stop one controller
	if err := manager.StopControllersForProviderConfig(ctx, pc2); err != nil {
		t.Fatalf("StopControllersForProviderConfig(%s) failed: %v", pc2.Name, err)
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
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
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
	manager.controllers.GetOrCreate(pc.Name)

	// Attempt start.
	// - Original code: existed=true, stopCh=nil -> start logic proceeds.
	// - Mutant code (existed || stopCh!=nil): -> returns early, skipping start.
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
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
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
	if _, exists := manager.controllers.Get(pc.Name); exists {
		t.Errorf("Controller map entry should have been removed after start failure")
	}
}

// TestManagerStartReturnsErrorOnNilChannel verifies that StartControllersForProviderConfig
// returns an error if the controller starter returns a nil channel and nil error.
func TestManagerStartReturnsErrorOnNilChannel(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
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

func toUnstructured(obj interface{}) *unstructured.Unstructured {
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		panic(fmt.Sprintf("failed to convert %T to unstructured: %v", obj, err))
	}
	return &unstructured.Unstructured{Object: content}
}
