package providerconfig

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	pcv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
	"github.com/google/go-cmp/cmp"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	testFinalizer    = "test-finalizer"
	testFinalizer2   = "test-finalizer2"
	defaultName      = "test-pc"
	defaultNamespace = "test-ns"
)

func setupFakeClient(initialPC *pcv1.ProviderConfig) *fake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	pcv1.AddToScheme(scheme)
	scheme.AddKnownTypes(ProviderConfigGVR.GroupVersion(), &pcv1.ProviderConfig{}, &pcv1.ProviderConfigList{})
	var objs []runtime.Object
	if initialPC != nil {
		objs = append(objs, initialPC)
	}
	return fake.NewSimpleDynamicClient(scheme, objs...)
}

func TestEnsureFinalizer(t *testing.T) {
	tests := []struct {
		name           string
		serverPC       *pcv1.ProviderConfig
		localPC        *pcv1.ProviderConfig
		finalizerToAdd string
		wantFinalizers []string
		wantError      bool
	}{
		{
			name: "add finalizer",
			serverPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultName,
					Namespace: defaultNamespace,
				},
			},
			localPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultName,
					Namespace: defaultNamespace,
				},
			},
			finalizerToAdd: testFinalizer,
			wantFinalizers: []string{testFinalizer},
		},
		{
			name: "finalizer already exists",
			serverPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       defaultName,
					Namespace:  defaultNamespace,
					Finalizers: []string{testFinalizer},
				},
			},
			localPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       defaultName,
					Namespace:  defaultNamespace,
					Finalizers: []string{testFinalizer},
				},
			},
			finalizerToAdd: testFinalizer,
			wantFinalizers: []string{testFinalizer},
		},
		{
			name: "preserve unrelated finalizer",
			serverPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       defaultName,
					Namespace:  defaultNamespace,
					Finalizers: []string{testFinalizer2},
				},
			},
			localPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       defaultName,
					Namespace:  defaultNamespace,
					Finalizers: []string{testFinalizer2},
				},
			},
			finalizerToAdd: testFinalizer,
			wantFinalizers: []string{testFinalizer2, testFinalizer},
		},
		{
			name: "add finalizer with stale local state (server has extra)",
			serverPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       defaultName,
					Namespace:  defaultNamespace,
					Finalizers: []string{testFinalizer2},
				},
			},
			localPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultName,
					Namespace: defaultNamespace,
					// Local state thinks there are no finalizers.
				},
			},
			finalizerToAdd: testFinalizer,
			wantFinalizers: []string{testFinalizer2, testFinalizer},
		},
		{
			name:           "object not found",
			serverPC:       nil,
			localPC:        &pcv1.ProviderConfig{ObjectMeta: metav1.ObjectMeta{Name: defaultName, Namespace: defaultNamespace}},
			finalizerToAdd: testFinalizer,
			wantFinalizers: []string{},
			wantError:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			dynamicClient := setupFakeClient(tc.serverPC)

			err := EnsureFinalizer(ctx, tc.localPC, dynamicClient, tc.finalizerToAdd)
			if gotErr := err != nil; gotErr != tc.wantError {
				t.Fatalf("EnsureFinalizer() returned error: %v, want error: %t", err, tc.wantError)
			}
			if tc.wantError {
				return
			}

			gotPC, err := dynamicClient.Resource(ProviderConfigGVR).Namespace(tc.localPC.Namespace).Get(ctx, tc.localPC.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get ProviderConfig: %v", err)
			}

			var gotProviderConfig pcv1.ProviderConfig
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(gotPC.UnstructuredContent(), &gotProviderConfig); err != nil {
				t.Fatalf("Failed to convert unstructured to ProviderConfig: %v", err)
			}

			if diff := cmp.Diff(tc.wantFinalizers, gotProviderConfig.Finalizers); diff != "" {
				t.Errorf("EnsureFinalizer() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEnsureFinalizer_ClientError(t *testing.T) {
	pc := &pcv1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultName,
			Namespace: defaultNamespace,
		},
	}
	client := setupFakeClient(pc)
	// Intercept Get to fail immediately
	client.PrependReactor("get", "providerconfigs", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("internal server error")
	})

	err := EnsureFinalizer(context.Background(), pc, client, "new-finalizer")
	if err == nil {
		t.Error("EnsureFinalizer() expected error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("EnsureFinalizer() unexpected error: %v", err)
	}
}

func TestEnsureFinalizer_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	pc := &pcv1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultName,
			Namespace: defaultNamespace,
		},
	}
	dynamicClient := setupFakeClient(pc)

	// Mock the client behavior: real clients return error if context is canceled.
	// Fake client doesn't do this automatically, so we inject a reactor.
	dynamicClient.PrependReactor("*", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		if ctx.Err() != nil {
			return true, nil, ctx.Err()
		}
		return false, nil, nil
	})

	err := EnsureFinalizer(ctx, pc, dynamicClient, testFinalizer)
	if err == nil {
		t.Fatal("Expected error when context is canceled, got nil")
	}
	// The exact error message depends on the client implementation, but it should typically contain "context canceled"
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected 'context canceled' error, got: %v", err)
	}
}

func TestDeleteFinalizer(t *testing.T) {
	tests := []struct {
		name              string
		serverPC          *pcv1.ProviderConfig
		localPC           *pcv1.ProviderConfig
		finalizerToDelete string
		wantFinalizers    []string
		wantError         bool
	}{
		{
			name: "remove finalizer",
			serverPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       defaultName,
					Namespace:  defaultNamespace,
					Finalizers: []string{testFinalizer},
				},
			},
			localPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       defaultName,
					Namespace:  defaultNamespace,
					Finalizers: []string{testFinalizer},
				},
			},
			finalizerToDelete: testFinalizer,
			wantFinalizers:    []string{},
		},
		{
			name: "remove with stale local state (server has extra)",
			serverPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       defaultName,
					Namespace:  defaultNamespace,
					Finalizers: []string{"A", testFinalizer, "C"},
				},
			},
			localPC: &pcv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:       defaultName,
					Namespace:  defaultNamespace,
					Finalizers: []string{testFinalizer}, // Stale: doesn't know about A and C
				},
			},
			finalizerToDelete: testFinalizer,
			wantFinalizers:    []string{"A", "C"},
		},
		{
			name:              "object not found",
			serverPC:          nil,
			localPC:           &pcv1.ProviderConfig{ObjectMeta: metav1.ObjectMeta{Name: defaultName, Namespace: defaultNamespace}},
			finalizerToDelete: testFinalizer,
			wantFinalizers:    []string{},
			wantError:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			dynamicClient := setupFakeClient(tc.serverPC)

			err := DeleteFinalizer(ctx, tc.localPC, dynamicClient, tc.finalizerToDelete)
			if gotErr := err != nil; gotErr != tc.wantError {
				t.Fatalf("DeleteFinalizer() returned error: %v, want error: %t", err, tc.wantError)
			}

			gotPC, err := dynamicClient.Resource(ProviderConfigGVR).Namespace(tc.localPC.Namespace).Get(ctx, tc.localPC.Name, metav1.GetOptions{})
			if tc.serverPC == nil {
				if !k8serrors.IsNotFound(err) {
					t.Errorf("Expected NotFound error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Failed to get ProviderConfig: %v", err)
			}

			var gotProviderConfig pcv1.ProviderConfig
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(gotPC.UnstructuredContent(), &gotProviderConfig); err != nil {
				t.Fatalf("Failed to convert unstructured to ProviderConfig: %v", err)
			}

			// Normalize nil vs empty slice for comparison
			if len(gotProviderConfig.Finalizers) == 0 && len(tc.wantFinalizers) == 0 {
				return
			}

			if diff := cmp.Diff(tc.wantFinalizers, gotProviderConfig.Finalizers); diff != "" {
				t.Errorf("DeleteFinalizer() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeleteFinalizer_ClientError(t *testing.T) {
	pc := &pcv1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       defaultName,
			Namespace:  defaultNamespace,
			Finalizers: []string{testFinalizer},
		},
	}
	client := setupFakeClient(pc)
	// Intercept Get to fail immediately
	client.PrependReactor("get", "providerconfigs", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("internal server error")
	})

	err := DeleteFinalizer(context.Background(), pc, client, testFinalizer)
	if err == nil {
		t.Error("DeleteFinalizer() expected error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("DeleteFinalizer() unexpected error: %v", err)
	}
}

func TestAddFinalizer_RaceCondition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverPC := &pcv1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       defaultName,
			Namespace:  defaultNamespace,
			Finalizers: []string{"A"},
		},
	}
	fakeClient := setupFakeClient(serverPC)

	// Simulate a race:
	// 1. Client GETs the object (sees "A").
	// 2. Client calls UPDATE (trying to set "A", "C").
	// 3. Reactor intercepts UPDATE.
	// 4. Reactor updates the underlying object to have "A", "B" (simulating another writer).
	// 5. Reactor returns Conflict error.
	// 6. Client retries -> GET (sees "A", "B") -> UPDATE (sets "A", "B", "C") -> Success.

	conflictTriggered := false

	fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		if conflictTriggered {
			// Second attempt, let it pass
			return false, nil, nil
		}
		conflictTriggered = true

		// Simulate the concurrent update
		tracker := fakeClient.Tracker()
		obj, _ := tracker.Get(ProviderConfigGVR, serverPC.Namespace, serverPC.Name)
		u, ok := obj.(metav1.Object)
		if !ok {
			return false, nil, nil
		}
		finalizers := u.GetFinalizers()
		u.SetFinalizers(append(finalizers, "B"))
		tracker.Update(ProviderConfigGVR, obj, serverPC.Namespace)

		// Return Conflict
		return true, nil, k8serrors.NewConflict(schema.GroupResource{Group: "providerconfig.k8s.io", Resource: "providerconfigs"}, serverPC.Name, fmt.Errorf("optimistic locking failure"))
	})

	err := AddFinalizer(ctx, serverPC, fakeClient, "C")
	if err != nil {
		t.Fatalf("AddFinalizer() failed: %v", err)
	}

	gotPC, _ := fakeClient.Resource(ProviderConfigGVR).Namespace(serverPC.Namespace).Get(ctx, serverPC.Name, metav1.GetOptions{})
	wantFinalizers := []string{"A", "B", "C"}
	if diff := cmp.Diff(wantFinalizers, gotPC.GetFinalizers()); diff != "" {
		t.Errorf("Finalizers mismatch after race (-want +got):\n%s", diff)
	}
}

func TestRemoveFinalizer_RaceCondition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverPC := &pcv1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       defaultName,
			Namespace:  defaultNamespace,
			Finalizers: []string{"A", "B", "C"},
		},
	}
	fakeClient := setupFakeClient(serverPC)

	// Simulate a race:
	// 1. Client GETs (sees "A", "B", "C").
	// 2. Client calls UPDATE (trying to set "A", "C" - removing "B").
	// 3. Reactor intercepts UPDATE.
	// 4. Reactor updates object to remove "A" -> "B", "C".
	// 5. Reactor returns Conflict.
	// 6. Client retries -> GET (sees "B", "C") -> UPDATE (sets "C" - removing "B") -> Success.

	conflictTriggered := false

	fakeClient.PrependReactor("update", "providerconfigs", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		if conflictTriggered {
			return false, nil, nil
		}
		conflictTriggered = true

		tracker := fakeClient.Tracker()
		obj, _ := tracker.Get(ProviderConfigGVR, serverPC.Namespace, serverPC.Name)
		u, ok := obj.(metav1.Object)
		if !ok {
			return false, nil, nil
		}
		finalizers := u.GetFinalizers()

		// Remove "A" (index 0)
		if len(finalizers) > 0 && finalizers[0] == "A" {
			u.SetFinalizers(finalizers[1:])
			tracker.Update(ProviderConfigGVR, obj, serverPC.Namespace)
		}

		return true, nil, k8serrors.NewConflict(schema.GroupResource{Group: "providerconfig.k8s.io", Resource: "providerconfigs"}, serverPC.Name, fmt.Errorf("optimistic locking failure"))
	})

	err := RemoveFinalizer(ctx, serverPC, fakeClient, "B")
	if err != nil {
		t.Fatalf("RemoveFinalizer() failed: %v", err)
	}

	gotPC, _ := fakeClient.Resource(ProviderConfigGVR).Namespace(serverPC.Namespace).Get(ctx, serverPC.Name, metav1.GetOptions{})
	wantFinalizers := []string{"C"} // A removed by reactor, B removed by function
	if diff := cmp.Diff(wantFinalizers, gotPC.GetFinalizers()); diff != "" {
		t.Errorf("Finalizers mismatch after successful race (-want +got):\n%s", diff)
	}
}
