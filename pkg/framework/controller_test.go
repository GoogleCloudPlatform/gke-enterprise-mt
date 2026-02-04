package framework

import (
	"context"
	"sync"
	"testing"
	"time"

	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"

	crv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/providerconfigcr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
)

func init() {
	// Register the ProviderConfig types with the scheme
	providerconfigv1.AddToScheme(scheme.Scheme)
}

// fakePCManager implements controllerManager
// and lets us track calls to StartControllersForProviderConfig/StopControllersForProviderConfig.
type fakePCManager struct {
	mu             sync.Mutex
	startedConfigs map[string]*providerconfigv1.ProviderConfig
	stoppedConfigs map[string]*providerconfigv1.ProviderConfig

	startErr error // optional injected error
	stopErr  error // optional injected error

	client        dynamic.Interface
	finalizerName string
}

func newFakeProviderConfigControllersManager(client dynamic.Interface, finalizerName string) *fakePCManager {
	return &fakePCManager{
		startedConfigs: make(map[string]*providerconfigv1.ProviderConfig),
		stoppedConfigs: make(map[string]*providerconfigv1.ProviderConfig),
		client:         client,
		finalizerName:  finalizerName,
	}
}

func (f *fakePCManager) StartControllersForProviderConfig(ctx context.Context, pc *providerconfigv1.ProviderConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startErr != nil {
		return f.startErr
	}
	// Add finalizer before recording start
	if err := crv1.EnsureFinalizer(ctx, pc, f.client, f.finalizerName); err != nil {
		return err
	}
	f.startedConfigs[pc.Name] = pc
	return nil
}

func (f *fakePCManager) StopControllersForProviderConfig(ctx context.Context, pc *providerconfigv1.ProviderConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopErr != nil {
		return f.stopErr
	}
	f.stoppedConfigs[pc.Name] = pc
	// Fetch latest PC and remove finalizer
	u, err := f.client.Resource(crv1.ProviderConfigGVR).Get(ctx, pc.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	latestPC, err := crv1.NewProviderConfig(u)
	if err != nil {
		return err
	}

	if err := crv1.DeleteFinalizer(ctx, latestPC, f.client, f.finalizerName); err != nil {
		return err
	}
	return nil
}

func (f *fakePCManager) HasStarted(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.startedConfigs[name]
	return ok
}

func (f *fakePCManager) HasStopped(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.stoppedConfigs[name]
	return ok
}

// wrapper that holds references to the controller under test plus some fakes
type testProviderConfigController struct {
	t      *testing.T
	stopCh chan struct{}

	manager      *fakePCManager
	pcController *Controller
	pcClient     dynamic.Interface
	pcInformer   *fakeInformer
}

// fakeInformer is a minimal implementation of cache.SharedIndexInformer for testing.
type fakeInformer struct {
	cache.Indexer
	synced  bool
	handler cache.ResourceEventHandler
}

func (f *fakeInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	f.handler = handler
	return nil, nil
}
func (f *fakeInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	f.handler = handler
	return nil, nil
}
func (f *fakeInformer) GetIndexer() cache.Indexer {
	return f.Indexer
}
func (f *fakeInformer) HasSynced() bool {
	return f.synced
}
func (f *fakeInformer) Run(stopCh <-chan struct{})                                 {}
func (f *fakeInformer) RunWithContext(ctx context.Context)                         {}
func (f *fakeInformer) GetStore() cache.Store                                      { return f.Indexer }
func (f *fakeInformer) LastSyncResourceVersion() string                            { return "" }
func (f *fakeInformer) SetWatchErrorHandler(handler cache.WatchErrorHandler) error { return nil }
func (f *fakeInformer) SetWatchErrorHandlerWithContext(handler func(context.Context, *cache.Reflector, error)) error {
	return nil
}
func (f *fakeInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	return nil
}
func (f *fakeInformer) IsStopped() bool                                            { return false }
func (f *fakeInformer) GetController() cache.Controller                            { return nil }
func (f *fakeInformer) SetTransform(handler cache.TransformFunc) error             { return nil }

func newTestProviderConfigController(t *testing.T) *testProviderConfigController {
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)

	fakeManager := newFakeProviderConfigControllersManager(dynamicClient, "test-finalizer")

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	fakeInformer := &fakeInformer{
		Indexer: indexer,
		synced:  true,
	}

	stopCh := make(chan struct{})

	ctrl := newController(
		fakeManager,
		fakeInformer,
		stopCh,
	)

	return &testProviderConfigController{
		t:            t,
		stopCh:       stopCh,
		pcController: ctrl,
		manager:      fakeManager,
		pcClient:     dynamicClient,
		pcInformer:   fakeInformer,
	}
}



func addProviderConfig(t *testing.T, tc *testProviderConfigController, pc *providerconfigv1.ProviderConfig) {
	t.Helper()
	// Update fake client
	if _, err := tc.pcClient.Resource(crv1.ProviderConfigGVR).Create(context.TODO(), toUnstructured(pc), metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create ProviderConfig: %v", err)
	}

	// Update indexer
	if err := tc.pcInformer.GetIndexer().Add(toUnstructured(pc)); err != nil {
		t.Fatalf("failed to add ProviderConfig to indexer: %v", err)
	}

	// Trigger handler
	if tc.pcInformer.handler != nil {
		tc.pcInformer.handler.OnAdd(toUnstructured(pc), false)
	}
}

func updateProviderConfig(t *testing.T, tc *testProviderConfigController, pc *providerconfigv1.ProviderConfig) {
	t.Helper()
	// Update fake client
	if _, err := tc.pcClient.Resource(crv1.ProviderConfigGVR).Update(context.TODO(), toUnstructured(pc), metav1.UpdateOptions{}); err != nil {
		t.Fatalf("failed to update ProviderConfig: %v", err)
	}

	// Update indexer
	if err := tc.pcInformer.GetIndexer().Update(toUnstructured(pc)); err != nil {
		t.Fatalf("failed to add ProviderConfig to indexer: %v", err)
	}

	// Trigger handler
	if tc.pcInformer.handler != nil {
		tc.pcInformer.handler.OnUpdate(nil, toUnstructured(pc))
	}
}

// TestStartAndStop verifies that the controller starts and stops gracefully when stopCh is closed.
func TestStartAndStop(t *testing.T) {
	tc := newTestProviderConfigController(t)

	// Start the controller in a separate goroutine
	controllerDone := make(chan struct{})
	go func() {
		tc.pcController.Run()
		close(controllerDone)
	}()

	// Signal stop immediately - we're testing graceful shutdown
	close(tc.stopCh)

	// Poll for graceful shutdown
	var err error
	if err = wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		select {
		case <-controllerDone:
			return true, nil
		default:
			return false, nil
		}
	}); err != nil {
		t.Fatal("Controller did not shut down within timeout")
	}

	if !tc.pcController.providerConfigQueue.ShuttingDown() {
		t.Error("Controller task queue did not shut down")
	}
}

func TestCreateDeleteProviderConfig(t *testing.T) {
	tc := newTestProviderConfigController(t)
	go tc.pcController.Run()
	defer close(tc.stopCh)

	pc := &providerconfigv1.ProviderConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ProviderConfig",
			APIVersion: "cloud.gke.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pc-delete",
		},
	}
	addProviderConfig(t, tc, pc)

	// Poll for manager to start the controller
	var err error
	if err = wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		return tc.manager.HasStarted("pc-delete"), nil
	}); err != nil {
		t.Errorf("Expected manager to have started 'pc-delete' within timeout: %v", err)
	}
	if tc.manager.HasStopped("pc-delete") {
		t.Errorf("Did not expect manager to have stopped 'pc-delete'")
	}

	// Now update it to have a DeletionTimestamp => triggers Stop
	pc2 := pc.DeepCopy()
	pc2.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	updateProviderConfig(t, tc, pc2)

	// Poll for manager to stop the controller
	if err = wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		return tc.manager.HasStopped("pc-delete"), nil
	}); err != nil {
		t.Errorf("Expected manager to stop 'pc-delete' within timeout: %v", err)
	}

	// Verify finalizer was removed.
	u, err := tc.pcClient.Resource(crv1.ProviderConfigGVR).Get(context.TODO(), pc.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get ProviderConfig: %v", err)
	}
	latestPC, err := crv1.NewProviderConfig(u)
	if err != nil {
		t.Errorf("failed to convert unstructured to ProviderConfig: %v", err)
	}
	if len(latestPC.Finalizers) > 0 {
		t.Errorf("Expected finalizers to be empty, got %v", latestPC.Finalizers)
	}
}

// TestCreateWithDeletionTimestamp verifies that if a ProviderConfig is created
// with DeletionTimestamp set, the controller calls StopControllersForProviderConfig instead of
// StartControllersForProviderConfig.
func TestCreateWithDeletionTimestamp(t *testing.T) {
	tc := newTestProviderConfigController(t)
	go tc.pcController.Run()
	defer close(tc.stopCh)

	pc := &providerconfigv1.ProviderConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ProviderConfig",
			APIVersion: "cloud.gke.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pc-deleted-on-create",
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
			Finalizers:        []string{"test-finalizer"},
		},
	}
	addProviderConfig(t, tc, pc)

	// Poll for manager to stop the controller
	var err error
	if err = wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		return tc.manager.HasStopped("pc-deleted-on-create"), nil
	}); err != nil {
		t.Errorf("Expected manager to stop 'pc-deleted-on-create' within timeout: %v", err)
	}

	// Verify it was NOT started
	if tc.manager.HasStarted("pc-deleted-on-create") {
		t.Errorf("Did not expect manager to have started 'pc-deleted-on-create'")
	}
}

// TestSyncNonExistent verifies that if the controller can't find the item in indexer, we return no
// error and do nothing.
func TestSyncNonExistent(t *testing.T) {
	tc := newTestProviderConfigController(t)
	go tc.pcController.Run()
	defer close(tc.stopCh)

	key := "some-ns/some-nonexistent"
	tc.pcController.providerConfigQueue.Enqueue(key)

	// Poll for queue to be empty, indicating the item was processed
	var err error
	if err = wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		return tc.pcController.providerConfigQueue.Len() == 0, nil
	}); err != nil {
		t.Fatalf("Queue did not become empty within timeout: %v", err)
	}

	// No starts or stops should have happened
	if len(tc.manager.startedConfigs) != 0 {
		t.Errorf("Unexpected StartControllersForProviderConfig call: %v", tc.manager.startedConfigs)
	}
	if len(tc.manager.stoppedConfigs) != 0 {
		t.Errorf("Did not expect manager to have stopped 'pc-delete'")
	}
}

// TestSyncBadObjectType ensures that if we get an unexpected type out of the indexer,
// we log an error but skip it.
func TestSyncBadObjectType(t *testing.T) {
	tc := newTestProviderConfigController(t)
	go tc.pcController.Run()
	defer close(tc.stopCh)

	// Insert something that is not *ProviderConfig
	// Here we expect NewProviderConfig to return an error.
	// So, create an unstructured object with a different GVK.
	unstructuredObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name": "not-a-pc",
			},
		},
	}
	// Note: if NewProviderConfig fails, the controller logs error and returns.
	// We want to verify it's not crashing or starting anything.

	// Add to indexer
	var err error
	if err = tc.pcInformer.GetIndexer().Add(unstructuredObj); err != nil {
		t.Fatalf("Failed to add unstructuredObj to indexer: %v", err)
	}

	// Trigger handler manually as informer doesn't watch dynamic client here.
	if tc.pcInformer.handler != nil {
		tc.pcInformer.handler.OnAdd(unstructuredObj, false)
	}

	// Poll for queue to be empty, indicating the item was processed
	if err = wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		return tc.pcController.providerConfigQueue.Len() == 0, nil
	}); err != nil {
		t.Fatalf("Queue did not become empty within timeout: %v", err)
	}

	if len(tc.manager.startedConfigs) != 0 {
		t.Errorf("Did not expect manager starts with a non-ProviderConfig object")
	}
	if len(tc.manager.stoppedConfigs) != 0 {
		t.Errorf("Did not expect manager stops with a non-ProviderConfig object")
	}
}

// fakePanickingManager implements controllerManager and panics on Start.
type fakePanickingManager struct {
	panicCounts map[string]int
	mu          sync.Mutex
}

func (f *fakePanickingManager) StartControllersForProviderConfig(ctx context.Context, pc *providerconfigv1.ProviderConfig) error {
	f.mu.Lock()
	if f.panicCounts == nil {
		f.panicCounts = make(map[string]int)
	}
	f.panicCounts[pc.Name]++
	f.mu.Unlock()
	panic("intentional panic for testing")
}

func (f *fakePanickingManager) StopControllersForProviderConfig(ctx context.Context, pc *providerconfigv1.ProviderConfig) error {
	return nil
}

func (f *fakePanickingManager) getPanicCount(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.panicCounts == nil {
		return 0
	}
	return f.panicCounts[name]
}

// TestPanicRecovery verifies that panics in sync are caught and don't crash the worker.
func TestPanicRecovery(t *testing.T) {
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
	panicManager := &fakePanickingManager{}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	fakeInformer := &fakeInformer{
		Indexer: indexer,
		synced:  true,
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	ctrl := newController(
		panicManager,
		fakeInformer,
		stopCh,
	)

	// Start controller in background
	go ctrl.Run()

	// Create a ProviderConfig that will trigger the panic
	pc := &providerconfigv1.ProviderConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ProviderConfig",
			APIVersion: "cloud.gke.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "panic-test",
		},
	}

	// Add to fake client
	var err error
	if _, err = dynamicClient.Resource(crv1.ProviderConfigGVR).Create(context.TODO(), toUnstructured(pc), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create ProviderConfig: %v", err)
	}
	// Add to indexer and trigger handler
	if err = indexer.Add(toUnstructured(pc)); err != nil {
		t.Fatalf("Failed to add ProviderConfig to indexer: %v", err)
	}
	if fakeInformer.handler != nil {
		fakeInformer.handler.OnAdd(toUnstructured(pc), false)
	}

	// Poll to verify the panic occurred but didn't crash the controller
	if err = wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		return panicManager.getPanicCount("panic-test") >= 1, nil
	}); err != nil {
		t.Errorf("expected panic to occur within timeout: %v", err)
	}

	// Verify the controller is still running by adding another ProviderConfig
	// If the worker crashed, this won't be processed
	pc2 := &providerconfigv1.ProviderConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ProviderConfig",
			APIVersion: "cloud.gke.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "after-panic",
		},
	}
	if _, err = dynamicClient.Resource(crv1.ProviderConfigGVR).Create(context.TODO(), toUnstructured(pc2), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create second ProviderConfig: %v", err)
	}
	// Add to indexer and trigger handler
	if err = indexer.Add(toUnstructured(pc2)); err != nil {
		t.Fatalf("Failed to add second ProviderConfig to indexer: %v", err)
	}
	if fakeInformer.handler != nil {
		fakeInformer.handler.OnAdd(toUnstructured(pc2), false)
	}

	// Poll for the second ProviderConfig to be processed (which will also panic).
	// If the controller crashed after the first panic, this won't be processed.
	if err = wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		return panicManager.getPanicCount("after-panic") >= 1, nil
	}); err != nil {
		t.Errorf("Expected second panic to occur within timeout (controller may have crashed): %v", err)
	}

	// Verify that the first item was retried (count > 1) because we return error on panic.
	// We poll because retry involves backoff.
	if err = wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		return panicManager.getPanicCount("panic-test") > 1, nil
	}); err != nil {
		t.Errorf("Expected retry for panic-test within timeout: %v", err)
	}

	t.Log("Controller survived panic and continued processing")
}