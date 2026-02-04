package providerconfig

import (
	"testing"

	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewProviderConfig(t *testing.T) {
	testcases := []struct {
		desc               string
		providerConfigCR   *unstructured.Unstructured
		wantProviderConfig *providerconfigv1.ProviderConfig
		wantErr            bool
	}{
		{
			desc: "valid_providerconfig",
			providerConfigCR: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "ProviderConfig",
					"apiVersion": "cloud.gke.io/v1",
					"metadata": map[string]any{
						"name": "test-pc",
					},
					"spec": map[string]any{
						"projectNumber": 1234567890,
						"projectID":     "test-project-1",
						"networkConfig": map[string]any{
							"network": "projects/test-project-1/global/networks/default",
							"subnetInfo": map[string]any{
								"subnetwork": "projects/test-project-1/regions/us-central1/subnetworks/default",
								"cidr":       "10.0.0.0/20",
								"podRanges": []any{
									map[string]any{"name": "pod-range-1", "cidr": "10.1.0.0/16"},
								},
							},
						},
					},
				},
			},
			wantProviderConfig: &providerconfigv1.ProviderConfig{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ProviderConfig",
					APIVersion: "cloud.gke.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pc",
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 1234567890,
					ProjectID:     "test-project-1",
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "projects/test-project-1/global/networks/default",
						SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
							Subnetwork: "projects/test-project-1/regions/us-central1/subnetworks/default",
							CIDR:       "10.0.0.0/20",
							PodRanges: []providerconfigv1.ProviderConfigSecondaryRange{
								{Name: "pod-range-1", CIDR: "10.1.0.0/16"},
							},
						},
					},
				},
			},
		},
		{
			desc: "incorrect_api_version",
			providerConfigCR: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "ProviderConfig",
					"apiVersion": "node",
					"metadata": map[string]any{
						"name": "test-pc",
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "incorrect_kind",
			providerConfigCR: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "node",
					"apiVersion": "cloud.gke.io/v1",
					"metadata": map[string]any{
						"name": "test-pc",
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "valid_providerconfig_missing_optional",
			providerConfigCR: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "ProviderConfig",
					"apiVersion": "cloud.gke.io/v1",
					"metadata": map[string]any{
						"name": "test-pc-no-optional",
					},
					"spec": map[string]any{
						"projectNumber": 1234567890,
						"projectID":     "test-project-1",
						"networkConfig": map[string]any{
							"network": "projects/test-project-1/global/networks/default",
							"subnetInfo": map[string]any{
								"subnetwork": "projects/test-project-1/regions/us-central1/subnetworks/default",
								"cidr":       "10.0.0.0/20",
							},
						},
					},
				},
			},
			wantProviderConfig: &providerconfigv1.ProviderConfig{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ProviderConfig",
					APIVersion: "cloud.gke.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pc-no-optional",
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 1234567890,
					ProjectID:     "test-project-1",
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "projects/test-project-1/global/networks/default",
						SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
							Subnetwork: "projects/test-project-1/regions/us-central1/subnetworks/default",
							CIDR:       "10.0.0.0/20",
						},
					},
				},
			},
		},
		{
			desc: "empty_spec",
			providerConfigCR: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "ProviderConfig",
					"apiVersion": "cloud.gke.io/v1",
					"metadata": map[string]any{
						"name": "test-pc-empty-spec",
					},
					"spec": map[string]any{},
				},
			},
			wantProviderConfig: &providerconfigv1.ProviderConfig{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ProviderConfig",
					APIVersion: "cloud.gke.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pc-empty-spec",
				},
				Spec: providerconfigv1.ProviderConfigSpec{},
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			pc, err := NewProviderConfig(tc.providerConfigCR)
			if tc.wantErr && err == nil {
				t.Errorf("NewProviderConfig(%v) = %v, want error", tc.providerConfigCR, pc)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("NewProviderConfig(%v) = %v, want nil", tc.providerConfigCR, err)
			}
			if diff := cmp.Diff(tc.wantProviderConfig, pc); diff != "" {
				t.Errorf("NewProviderConfig(%v) returned diff (-want +got):\n%s", tc.providerConfigCR, diff)
			}
		})
	}
}
