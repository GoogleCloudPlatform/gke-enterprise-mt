package gce

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
)

func TestGCEConfForProviderConfig(t *testing.T) {
	testCases := []struct {
		name           string
		gceConf        Config
		providerConfig *providerconfigv1.ProviderConfig
		wantConfig     Config
		wantErr        bool
	}{
		{
			name: "nil provider config returns default config",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
			wantConfig: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
		},
		{
			name: "quoted token body, nil provider config, returns quoted config",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `"{\"projectNumber\":12345,\"clusterId\":\"example-cluster\"}"`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
			wantConfig: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `"{\"projectNumber\":12345,\"clusterId\":\"example-cluster\"}"`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
		},
		{
			name: "provider config for supervisor with AuthConfig updates token info",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
			providerConfig: &providerconfigv1.ProviderConfig{
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "new-project",
					ProjectNumber: 12345,
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "new-network",
						SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
							Subnetwork: "new-subnet",
						},
					},
					AuthConfig: &providerconfigv1.AuthConfig{
						TokenURL:  "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/supervisor-cluster:generateToken",
						TokenBody: `{"projectNumber":12345,"clusterId":"supervisor-cluster"}`,
					},
				},
			},
			wantConfig: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/supervisor-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":12345,"clusterId":"supervisor-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "supervisor-cluster",
					},
					ProjectID:        "new-project",
					NetworkName:      "new-network",
					NetworkProjectID: "new-project",
					NetworkURL:       "new-subnet",
				},
			},
		},
		{
			name: "provider config with tenant label updates config",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-tenant"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "new-project",
					ProjectNumber: 67890,
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "new-network",
						SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
							Subnetwork: "new-subnet",
						},
					},
				},
			},
			wantConfig: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/67890/locations/us-central1/tenants/example-tenant:generateTenantToken",
					TokenBodyRaw: `{"projectNumber":67890,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 67890,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "new-project",
					NetworkName:      "new-network",
					NetworkProjectID: "new-project",
					NetworkURL:       "new-subnet",
				},
			},
		},
		{
			name: "invalid token body returns error",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-tenant"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "new-project",
					ProjectNumber: 67890,
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "new-network",
						SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
							Subnetwork: "new-subnet",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "provider config with AuthConfig for tenant",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant-name",
					Labels: map[string]string{tenantLabelKey: "example-tenant-name"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "tenant-project",
					ProjectNumber: 99999,
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "tenant-network",
						SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
							Subnetwork: "tenant-subnet",
						},
					},
					AuthConfig: &providerconfigv1.AuthConfig{
						TokenURL:  "https://gkeauth.googleapis.com/v1/projects/99999/locations/us-central1/tenants/example-tenant-name:generateTenantToken",
						TokenBody: `{"projectNumber":99999,"clusterId":"example-cluster"}`,
					},
				},
			},
			wantConfig: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/99999/locations/us-central1/tenants/example-tenant-name:generateTenantToken",
					TokenBodyRaw: `{"projectNumber":99999,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 99999,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "tenant-project",
					NetworkName:      "tenant-network",
					NetworkProjectID: "tenant-project",
					NetworkURL:       "tenant-subnet",
				},
			},
		},
		{
			name: "provider config with AuthConfig for supervisor",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "s77777",
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "supervisor-project",
					ProjectNumber: 77777,
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "supervisor-network",
						SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
							Subnetwork: "supervisor-subnet",
						},
					},
					AuthConfig: &providerconfigv1.AuthConfig{
						TokenURL:  "https://gkeauth.googleapis.com/v1/projects/77777/locations/us-central1/clusters/the-supervisor-cluster:generateToken",
						TokenBody: `{"projectNumber":77777,"clusterId":"the-supervisor-cluster"}`,
					},
				},
			},
			wantConfig: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/77777/locations/us-central1/clusters/the-supervisor-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":77777,"clusterId":"the-supervisor-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 77777,
						ClusterID:     "the-supervisor-cluster",
					},
					ProjectID:        "supervisor-project",
					NetworkName:      "supervisor-network",
					NetworkProjectID: "supervisor-project",
					NetworkURL:       "supervisor-subnet",
				},
			},
		},
		{
			name: "provider config with empty AuthConfig fields triggers fallback",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-tenant"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "new-project",
					ProjectNumber: 67890,
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "new-network",
						SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
							Subnetwork: "new-subnet",
						},
					},
					AuthConfig: &providerconfigv1.AuthConfig{
						TokenURL:  "",
						TokenBody: "",
					},
				},
			},
			wantConfig: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/67890/locations/us-central1/tenants/example-tenant:generateTenantToken",
					TokenBodyRaw: `{"projectNumber":67890,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 67890,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "new-project",
					NetworkName:      "new-network",
					NetworkProjectID: "new-project",
					NetworkURL:       "new-subnet",
				},
			},
		},
		{
			name: "provider config with malformed JSON in AuthConfig TokenBody",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:   "default-project",
					NetworkName: "default",
				},
			},
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "error-tenant",
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "error-project",
					ProjectNumber: 88888,
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "error-network",
					},
					AuthConfig: &providerconfigv1.AuthConfig{
						TokenURL:  "https://gkeauth.googleapis.com/v1/projects/88888/locations/us-central1/tenants/error-tenant:generateTenantToken",
						TokenBody: `{"projectNumber": 88888, "clusterId"}`,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AuthConfig TokenBody without clusterId does not retain default clusterId and preserves custom projectNumber",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
					TokenBody: &TokenBody{
						ProjectNumber: 12345,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "custom-auth-tenant",
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "tenant-project",
					ProjectNumber: 99999,
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "tenant-network",
						SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
							Subnetwork: "tenant-subnet",
						},
					},
					AuthConfig: &providerconfigv1.AuthConfig{
						TokenURL:  "https://gkeauth.googleapis.com/v1/projects/55555/locations/us-central1/tenants/custom-auth-tenant:generateTenantToken",
						TokenBody: `{"projectNumber":55555}`,
					},
				},
			},
			wantConfig: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/55555/locations/us-central1/tenants/custom-auth-tenant:generateTenantToken",
					TokenBodyRaw: `{"projectNumber":55555}`,
					TokenBody: &TokenBody{
						ProjectNumber: 55555,
						ClusterID:     "",
					},
					ProjectID:        "tenant-project",
					NetworkName:      "tenant-network",
					NetworkProjectID: "tenant-project",
					NetworkURL:       "tenant-subnet",
				},
			},
		},
		{
			name: "empty TokenBodyRaw preserves existing TokenBody fields and fills ProjectNumber",
			gceConf: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: "",
					TokenBody: &TokenBody{
						ClusterID: "example-cluster",
					},
					ProjectID:        "default-project",
					NetworkName:      "default",
					NetworkProjectID: "default-project",
					NetworkURL:       "default",
				},
			},
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "s77777",
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "supervisor-project",
					ProjectNumber: 77777,
					NetworkConfig: providerconfigv1.ProviderNetworkConfig{
						Network: "supervisor-network",
						SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
							Subnetwork: "supervisor-subnet",
						},
					},
				},
			},
			wantConfig: Config{
				Global: Global{
					TokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
					TokenBodyRaw: "",
					TokenBody: &TokenBody{
						ProjectNumber: 77777,
						ClusterID:     "example-cluster",
					},
					ProjectID:        "supervisor-project",
					NetworkName:      "supervisor-network",
					NetworkProjectID: "supervisor-project",
					NetworkURL:       "supervisor-subnet",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			creator := NewDefaultConfCreator(&tc.gceConf)

			gotConfig, err := creator.GCEConfForProviderConfig(tc.providerConfig)
			if (err != nil) != tc.wantErr {
				t.Errorf("GCEConfForProviderConfig(%v) returned error: %v, want error: %t", tc.providerConfig, err, tc.wantErr)
			}
			if err != nil {
				return
			}
			tc.wantConfig.Global.TokenBodyRaw = mustNormalizeJSON(t, tc.wantConfig.Global.TokenBodyRaw)
			gotConfig.Global.TokenBodyRaw = mustNormalizeJSON(t, gotConfig.Global.TokenBodyRaw)
			if diff := cmp.Diff(&tc.wantConfig, gotConfig); diff != "" {
				t.Errorf("GCEConfForProviderConfig(%v) returned diff (-want +got):\n%s", tc.providerConfig, diff)
			}
		})
	}
}

func mustNormalizeJSON(t *testing.T, jsonString string) string {
	t.Helper()
	if jsonString == "" {
		return ""
	}

	unquoted, isQuoted, err := unquoteJSON(jsonString)
	if err != nil {
		t.Fatalf("Invalid quoted JSON string: %v", err)
	}

	var temp any
	if err := json.Unmarshal([]byte(unquoted), &temp); err != nil {
		t.Fatalf("Invalid JSON string: %v", err)
	}

	normalizedBytes, err := json.Marshal(temp)
	if err != nil {
		t.Fatalf("Error marshaling JSON: %v", err)
	}

	normalizedString := string(normalizedBytes)

	if isQuoted {
		normalizedString = "\"" + strings.ReplaceAll(normalizedString, "\"", "\\\"") + "\""
	}

	return normalizedString
}

func TestDefaultConfigImmutability(t *testing.T) {
	originalTokenURL := "https://gkeauth.googleapis.com/v1/projects/123/locations/us-east1/clusters/original-cluster:generateToken"
	originalGCEConf := &Config{
		Global: Global{
			TokenURL:     originalTokenURL,
			TokenBodyRaw: `{"projectNumber":123,"clusterId":"original-id"}`,
			TokenBody: &TokenBody{
				ProjectNumber: 123,
				ClusterID:     "original-id",
			},
			ProjectID:        "original-project",
			NetworkName:      "original-network",
			NetworkProjectID: "original-project",
			NetworkURL:       "original-subnet",
		},
	}

	expectedDefaultTokenBody := &TokenBody{
		ProjectNumber: originalGCEConf.Global.TokenBody.ProjectNumber,
		ClusterID:     originalGCEConf.Global.TokenBody.ClusterID,
	}
	expectedDefaultTokenBodyRaw := originalGCEConf.Global.TokenBodyRaw
	expectedDefaultProjectID := originalGCEConf.Global.ProjectID
	expectedDefaultNetworkName := originalGCEConf.Global.NetworkName
	expectedDefaultNetworkProjectID := originalGCEConf.Global.NetworkProjectID
	expectedDefaultNetworkURL := originalGCEConf.Global.NetworkURL

	creator := NewDefaultConfCreator(originalGCEConf)

	providerConfigToMutate := &providerconfigv1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-tenant",
			Labels: map[string]string{tenantLabelKey: "test-tenant"},
		},
		Spec: providerconfigv1.ProviderConfigSpec{
			ProjectID:     "modified-project",
			ProjectNumber: 789,
			NetworkConfig: providerconfigv1.ProviderNetworkConfig{
				Network: "projects/modified-project/global/networks/modified-network",
				SubnetInfo: providerconfigv1.ProviderConfigSubnetInfo{
					Subnetwork: "projects/modified-project/regions/us-east1/subnetworks/modified-subnet",
				},
			},
		},
	}

	if _, err := creator.GCEConfForProviderConfig(providerConfigToMutate); err != nil {
		t.Fatalf("GCEConfForProviderConfig with providerConfigToMutate failed: %v", err)
	}

	if creator.defaultConfig.Global.TokenBody.ProjectNumber != expectedDefaultTokenBody.ProjectNumber {
		t.Errorf("Default config TokenBody.ProjectNumber was mutated: got %d, want %d",
			creator.defaultConfig.Global.TokenBody.ProjectNumber, expectedDefaultTokenBody.ProjectNumber)
	}

	secondCallConfig, err := creator.GCEConfForProviderConfig(nil)
	if err != nil {
		t.Fatalf("GCEConfForProviderConfig with nil providerConfig failed: %v", err)
	}

	if diff := cmp.Diff(expectedDefaultTokenBody, secondCallConfig.Global.TokenBody); diff != "" {
		t.Errorf("GCEConfForProviderConfig(nil) after modification returned diff in TokenBody (-want +got):\n%s", diff)
	}
	if gotRaw, wantRaw := mustNormalizeJSON(t, secondCallConfig.Global.TokenBodyRaw), mustNormalizeJSON(t, expectedDefaultTokenBodyRaw); gotRaw != wantRaw {
		t.Errorf("GCEConfForProviderConfig(nil) after modification returned TokenBodyRaw: got %q, want %q", gotRaw, wantRaw)
	}
	if secondCallConfig.Global.TokenURL != originalTokenURL {
		t.Errorf("GCEConfForProviderConfig(nil) after modification returned TokenURL: got %q, want %q", secondCallConfig.Global.TokenURL, originalTokenURL)
	}
	if secondCallConfig.Global.ProjectID != expectedDefaultProjectID {
		t.Errorf("GCEConfForProviderConfig(nil) after modification returned ProjectID: got %q, want %q", secondCallConfig.Global.ProjectID, expectedDefaultProjectID)
	}
	if secondCallConfig.Global.NetworkName != expectedDefaultNetworkName {
		t.Errorf("GCEConfForProviderConfig(nil) after modification returned NetworkName: got %q, want %q", secondCallConfig.Global.NetworkName, expectedDefaultNetworkName)
	}
	if secondCallConfig.Global.NetworkProjectID != expectedDefaultNetworkProjectID {
		t.Errorf("GCEConfForProviderConfig(nil) after modification returned NetworkProjectID: got %q, want %q", secondCallConfig.Global.NetworkProjectID, expectedDefaultNetworkProjectID)
	}
	if secondCallConfig.Global.NetworkURL != expectedDefaultNetworkURL {
		t.Errorf("GCEConfForProviderConfig(nil) after modification returned NetworkURL: got %q, want %q", secondCallConfig.Global.NetworkURL, expectedDefaultNetworkURL)
	}
}
