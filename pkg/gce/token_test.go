package gce

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
)

func TestTokenURLForProviderConfig(t *testing.T) {
	testCases := []struct {
		name           string
		tokenURL       string
		providerConfig *providerconfigv1.ProviderConfig

		wantTokenURL string
		wantErr      bool
	}{
		{
			name:         "nil provider config returns default config",
			tokenURL:     "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
			wantTokenURL: "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
		},
		{
			name:     "does not update when no tenant label is present",
			tokenURL: "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
			providerConfig: &providerconfigv1.ProviderConfig{
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "new-project",
					ProjectNumber: 67890,
				},
			},
			wantTokenURL: "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
		},
		{
			name:     "provider config with name and tenant label updates tokenURL to generateTenantToken",
			tokenURL: "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-tenant"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 67890,
				},
			},
			wantTokenURL: "https://gkeauth.googleapis.com/v1/projects/67890/locations/us-central1/tenants/example-tenant:generateTenantToken",
		},
		{
			name:     "provider config with name but no tenant label updates project, keeps original path structure",
			tokenURL: "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-tenant",
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 67890,
				},
			},
			wantTokenURL: "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
		},
		{
			name:     "malformed tokenURL for project replacement returns error",
			tokenURL: "https://gkeauth.googleapis.com/v1/projectswithoutslashlocations/us-central1/clusters/example-cluster:generateToken",
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-tenant"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 67890,
				},
			},
			wantErr: true,
		},
		{
			name:     "provider config with name and owner label, non-standard base URL, updates to tenant URL",
			tokenURL: "https://staging-gkeauth.googleapis.com/v60/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-owner"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 67890,
				},
			},
			wantTokenURL: "https://staging-gkeauth.googleapis.com/v60/projects/67890/locations/us-central1/tenants/example-tenant:generateTenantToken",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotTokenURL, err := TokenURLForProviderConfig(tc.tokenURL, tc.providerConfig)
			if (err != nil) != tc.wantErr {
				t.Errorf("TokenURLForProviderConfig(tokenURL=%q, providerConfig=%+v) returned error: %v, wantErr: %t", tc.tokenURL, tc.providerConfig, err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if gotTokenURL != tc.wantTokenURL {
				t.Errorf("TokenURLForProviderConfig(tokenURL=%q, providerConfig=%+v) = %q, want %q", tc.tokenURL, tc.providerConfig, gotTokenURL, tc.wantTokenURL)
			}
		})
	}
}

func TestTokenURLForTenantProject(t *testing.T) {
	testCases := []struct {
		name          string
		tokenURL      string
		tenantProject string
		wantError     bool
		wantTokenURL  string
	}{
		{
			name:          "invalid_existing_token_url",
			tokenURL:      "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters",
			tenantProject: "123456",
			wantError:     true,
		},
		{
			name:          "valid_existing_token_url",
			tokenURL:      "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster:generateToken",
			tenantProject: "123456",
			wantTokenURL:  "https://gkeauth.googleapis.com/v1/projects/12345/locations/us-central1/clusters/example-cluster/tenantProjects/123456:generateTenantProjectToken",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotTokenURL, err := TokenURLForTenantProject(tc.tokenURL, tc.tenantProject)
			if (err != nil) != tc.wantError {
				t.Errorf("TokenURLForTenantProject(tokenURL=%q, tenantProject=%q) returned error: %v, wantErr: %t", tc.tokenURL, tc.tenantProject, err, tc.wantError)
			}
			if err != nil {
				return
			}
			if gotTokenURL != tc.wantTokenURL {
				t.Errorf("TokenURLForTenantProject(tokenURL=%q, tenantProject=%q) = %q, want %q", tc.tokenURL, tc.tenantProject, gotTokenURL, tc.wantTokenURL)
			}
		})
	}
}

func TestTokenBodyForProviderConfig(t *testing.T) {
	testCases := []struct {
		name           string
		tokenBody      string
		providerConfig *providerconfigv1.ProviderConfig

		wantTokenBody string
		wantErr       bool
	}{
		{
			name:          "nil provider config returns default token body",
			tokenBody:     `{"projectNumber":12345,"clusterId":"example-cluster"}`,
			wantTokenBody: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
		},
		{
			name:          "quoted token body, nil provider config, returns quoted config",
			tokenBody:     `"{\"projectNumber\":12345,\"clusterId\":\"example-cluster\"}"`,
			wantTokenBody: `"{\"projectNumber\":12345,\"clusterId\":\"example-cluster\"}"`,
		},
		{
			name:      "does not update when no tenant label is present",
			tokenBody: `"{\"projectNumber\":12345,\"clusterId\":\"example-cluster\"}"`,
			providerConfig: &providerconfigv1.ProviderConfig{
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectID:     "new-project",
					ProjectNumber: 67890,
				},
			},
			wantTokenBody: `"{\"projectNumber\":12345,\"clusterId\":\"example-cluster\"}"`,
		},
		{
			name:      "provider config with name and tenant label updates token body",
			tokenBody: `"{\"projectNumber\":12345,\"clusterId\":\"example-cluster\"}"`,
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-tenant"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 67890,
				},
			},
			wantTokenBody: `"{\"projectNumber\":67890,\"clusterId\":\"example-cluster\"}"`,
		},
		{
			name:      "provider config with name and tenant label updates unquoted token body",
			tokenBody: `{"projectNumber":12345,"clusterId":"example-cluster"}`,
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-tenant"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 67890,
				},
			},
			wantTokenBody: `{"projectNumber":67890,"clusterId":"example-cluster"}`,
		},
		{
			name:      "provider config with name and tenant label adds project number if missing",
			tokenBody: `"{\"clusterId\":\"example-cluster\"}"`,
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-tenant"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 67890,
				},
			},
			wantTokenBody: `"{\"projectNumber\":67890,\"clusterId\":\"example-cluster\"}"`,
		},
		{
			name:      "malformed token body returns error",
			tokenBody: `{"clusterId":"example-cluster"`,
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-tenant"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 67890,
				},
			},
			wantErr: true,
		},
		{
			name:      "works with nested json",
			tokenBody: `{"data": {"random-field": "qwerty"},"clusterId":"example-cluster"}`,
			providerConfig: &providerconfigv1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "example-tenant",
					Labels: map[string]string{tenantLabelKey: "example-owner"},
				},
				Spec: providerconfigv1.ProviderConfigSpec{
					ProjectNumber: 67890,
				},
			},
			wantTokenBody: `{"clusterId":"example-cluster","data":{"random-field":"qwerty"},"projectNumber":67890}`,
			wantErr:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := TokenBodyForProviderConfig(tc.tokenBody, tc.providerConfig)
			if tc.wantErr {
				if err == nil {
					t.Errorf("TokenBodyForProviderConfig(%q, %+v) = %q, want error", tc.tokenBody, tc.providerConfig, result)
				}
				return
			}
			if err != nil {
				t.Errorf("TokenBodyForProviderConfig(%q, %+v) returned unexpected error: %v", tc.tokenBody, tc.providerConfig, err)
				return
			}
			expectedJSON := mustNormalizeJSON(t, tc.wantTokenBody)
			resultJSON := mustNormalizeJSON(t, result)

			if resultJSON != expectedJSON {
				t.Errorf("TokenBodyForProviderConfig(%q, %+v) = %s, want %s", tc.tokenBody, tc.providerConfig, resultJSON, expectedJSON)
			}
		})
	}
}
