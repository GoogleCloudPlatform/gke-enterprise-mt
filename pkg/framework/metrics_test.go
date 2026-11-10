package framework

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	mtmetrics "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"
)

func TestControllerNameFromFinalizer(t *testing.T) {
	testCases := []struct {
		name          string
		finalizerName string
		want          string
		wantErr       bool
	}{
		{
			name:          "valid neg finalizer",
			finalizerName: "multiproject.networking.gke.io/neg-cleanup",
			want:          "neg",
		},
		{
			name:          "valid clouddns finalizer",
			finalizerName: "multiproject.networking.gke.io/clouddns-cleanup",
			want:          "clouddns",
		},
		{
			name:          "valid ccm finalizer",
			finalizerName: "multiproject.networking.gke.io/ccm-cleanup",
			want:          "ccm",
		},
		{
			name:          "valid hyphenated controller name",
			finalizerName: "multiproject.networking.gke.io/my-custom-ctrl-cleanup",
			want:          "my-custom-ctrl",
		},
		{
			name:          "empty finalizer",
			finalizerName: "",
			wantErr:       true,
		},
		{
			name:          "missing domain prefix",
			finalizerName: "neg-cleanup",
			wantErr:       true,
		},
		{
			name:          "empty domain prefix",
			finalizerName: "/neg-cleanup",
			wantErr:       true,
		},
		{
			name:          "missing cleanup suffix",
			finalizerName: "multiproject.networking.gke.io/custom-finalizer",
			wantErr:       true,
		},
		{
			name:          "empty controller name before cleanup suffix",
			finalizerName: "multiproject.networking.gke.io/-cleanup",
			wantErr:       true,
		},
		{
			name:          "extra slash in controller name",
			finalizerName: "multiproject.networking.gke.io/sub/neg-cleanup",
			wantErr:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := controllerNameFromFinalizer(tc.finalizerName)
			if (err != nil) != tc.wantErr {
				t.Fatalf("controllerNameFromFinalizer(%q) error = %v, wantErr %v", tc.finalizerName, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("controllerNameFromFinalizer(%q) = %q, want %q", tc.finalizerName, got, tc.want)
			}
		})
	}
}

func TestSetControllerRunningMetricInvalidFinalizerSkipsEmission(t *testing.T) {
	ctx := context.Background()
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil)
	mockStarter := newMockControllerStarter()

	const (
		invalidFinalizer = "multiproject.networking.gke.io/custom-finalizer"
		pcName           = "test-pc-invalid-finalizer"
		tenantUID        = "tenant-uid-invalid-finalizer"
	)
	defer mtmetrics.CleanupTenant(tenantUID)

	mgr := newManager(dynamicClient, invalidFinalizer, mockStarter)

	pc := createTestProviderConfig(pcName)
	if err := unstructured.SetNestedField(pc.Object, tenantUID, "spec", "principalInfo", "id"); err != nil {
		t.Fatalf("Failed to set tenant UID: %v", err)
	}
	if err := createProviderConfigInClient(ctx, dynamicClient, pc); err != nil {
		t.Fatalf("Failed to create test ProviderConfig: %v", err)
	}

	if err := mgr.StartControllersForProviderConfig(ctx, pc); err != nil {
		t.Fatalf("StartControllersForProviderConfig failed: %v", err)
	}

	mg, ok := mtmetrics.DefaultMultiGatherer.(*mtmetrics.MultiGatherer)
	if !ok {
		t.Fatalf("Expected DefaultMultiGatherer to be *mtmetrics.MultiGatherer")
	}
	mfs, err := mg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather from DefaultMultiGatherer: %v", err)
	}
	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "tenant_uid" && l.GetValue() == tenantUID {
					t.Errorf("Expected no metrics emitted for tenant %q when finalizer %q is invalid and controllerName is empty, found %s", tenantUID, invalidFinalizer, mf.GetName())
				}
			}
		}
	}
}
