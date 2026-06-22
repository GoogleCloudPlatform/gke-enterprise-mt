package framework

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/klog/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// manager coordinates lifecycle of controllers scoped to individual ProviderConfigs.
// It ensures per-ProviderConfig controller startup is idempotent, adds/removes
// finalizers, and wires stop channels for clean shutdown.
//
// This manager assumes it is invoked by a workqueue that guarantees
// the same ProviderConfig key is never processed concurrently.
type manager struct {
	controllers *ControllerMap

	client            dynamic.Interface
	finalizerName     string
	controllerStarter ControllerStarter
}

// newManager constructs a new generic ProviderConfig controller manager.
// It does not start any controllers until StartControllersForProviderConfig is invoked.
func newManager(client dynamic.Interface, finalizerName string, controllerStarter ControllerStarter,
) *manager {
	return &manager{
		controllers:       NewControllerMap(),
		client:            client,
		finalizerName:     finalizerName,
		controllerStarter: controllerStarter,
	}
}

var providerConfigGVR = schema.GroupVersionResource{
	Group:    "cloud.gke.io",
	Version:  "v1",
	Resource: "providerconfigs",
}

var providerConfigGVK = schema.GroupVersionKind{
	Group:   "cloud.gke.io",
	Version: "v1",
	Kind:    "ProviderConfig",
}

// providerConfigKey returns the key for a ProviderConfig in the controller map.
func providerConfigKey(pc *unstructured.Unstructured) string {
	return pc.GetName()
}

func (m *manager) getProviderConfig(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	return m.client.Resource(providerConfigGVR).Get(ctx, name, metav1.GetOptions{})
}

// rollbackFinalizerOnStartFailure removes the finalizer after a start failure
// so that ProviderConfig deletion is not blocked.
func (m *manager) rollbackFinalizerOnStartFailure(ctx context.Context, pc *unstructured.Unstructured, cause error) {
	pcLatest, err := m.getProviderConfig(ctx, pc.GetName())
	if err != nil {
		klog.Errorf("failed to get latest ProviderConfig for finalizer rollback: %v, originalError: %v", err, cause)
		return
	}
	finalizers := pcLatest.GetFinalizers()
	newFinalizers := slices.DeleteFunc(finalizers, func(f string) bool { return f == m.finalizerName })
	if len(newFinalizers) != len(finalizers) {
		pcLatest.SetFinalizers(newFinalizers)
		_, err := m.client.Resource(providerConfigGVR).Update(ctx, pcLatest, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("failed to clean up finalizer after start failure: %v, originalError: %v", err, cause)
		}
	}
}

// StartControllersForProviderConfig ensures finalizers are present and starts
// the controllers associated with the given ProviderConfig. The call is
// idempotent: repeated calls for the same ProviderConfig will only start
// controllers once.
func (m *manager) StartControllersForProviderConfig(ctx context.Context, pc *unstructured.Unstructured) error {
	if pc.GroupVersionKind() != providerConfigGVK {
		return fmt.Errorf("expected object of kind %s, but got %s", providerConfigGVK, pc.GroupVersionKind())
	}
	if !pc.GetDeletionTimestamp().IsZero() {
		klog.InfoDepth(3, "ProviderConfig is terminating; skipping start")
		return nil
	}

	pcKey := providerConfigKey(pc)

	cs, existed := m.controllers.GetOrCreate(pcKey)
	if cs.stopCh != nil {
		klog.Info("Controllers for provider config already exist, skipping start")
		return nil
	}

	klog.Info("Starting controllers for provider config")

	finalizers := pc.GetFinalizers()
	hadFinalizer := slices.Contains(finalizers, m.finalizerName)

	if !hadFinalizer {
		pc.SetFinalizers(append(finalizers, m.finalizerName))
		_, err := m.client.Resource(providerConfigGVR).Update(ctx, pc, metav1.UpdateOptions{})
		if err != nil {
			if !existed {
				m.controllers.Delete(pcKey)
			}
			return fmt.Errorf("failed to ensure finalizer %s for provider config %s: %w", m.finalizerName, pcKey, err)
		}
	}

	controllerStopCh, err := m.controllerStarter.StartController(pc)
	if err == nil && controllerStopCh == nil {
		err = fmt.Errorf("controller starter returned nil channel")
	}
	if err != nil {
		if !existed {
			m.controllers.Delete(pcKey)
		}
		if !hadFinalizer {
			m.rollbackFinalizerOnStartFailure(ctx, pc, err)
		}
		return fmt.Errorf("failed to start controller for provider config %s: %w", pcKey, err)
	}

	cs.stopCh = controllerStopCh

	klog.Info("Started controllers for provider config")
	return nil
}

// StopControllersForProviderConfig stops the controllers for the given ProviderConfig
// and removes the associated finalizer. Finalizer removal is attempted even if no
// controller mapping exists, ensuring deletion can proceed after process restarts
// or when controllers were previously stopped.
func (m *manager) StopControllersForProviderConfig(ctx context.Context, pc *unstructured.Unstructured) error {
	if pc.GroupVersionKind() != providerConfigGVK {
		return fmt.Errorf("expected object of kind %s, but got %s", providerConfigGVK, pc.GroupVersionKind())
	}
	pcKey := providerConfigKey(pc)

	if cs, exists := m.controllers.Get(pcKey); exists {
		m.controllers.Delete(pcKey)
		if cs.stopCh != nil {
			close(cs.stopCh)
			klog.Info("Signaled controller stop")
		} else {
			klog.Info("Controllers for provider config already stopped")
		}
	} else {
		klog.Info("Controllers for provider config do not exist")
	}

	// Fetch the latest ProviderConfig to ensure we have current finalizer state.
	latestPC, err := m.getProviderConfig(ctx, pc.GetName())
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Info("ProviderConfig not found while stopping controllers; skipping finalizer removal")
			return nil
		}
		return fmt.Errorf("Failed to get latest ProviderConfig for finalizer removal: %w", err)
	}

	finalizers := latestPC.GetFinalizers()
	newFinalizers := slices.DeleteFunc(finalizers, func(f string) bool { return f == m.finalizerName })
	if len(newFinalizers) != len(finalizers) {
		latestPC.SetFinalizers(newFinalizers)
		_, err := m.client.Resource(providerConfigGVR).Update(ctx, latestPC, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("Failed to delete finalizer %s for provider config %s: %w", m.finalizerName, pcKey, err)
		}
	}
	klog.Info("Stopped controllers for provider config")
	return nil
}
