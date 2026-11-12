package gce

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
)

const (
	tenantLabelKey = "tenancy.gke.io/tenant"
)

// TokenURLForProviderConfig generates the token URL based on the provider configuration.
// It first checks for the new AuthConfig spec field. If present, it returns the TokenURL from there.
// As a fallback for backward compatibility, it checks for a tenant label to determine
// if the token URL should be for a tenant project.
func TokenURLForProviderConfig(existingTokenURL string, providerConfig *providerconfigv1.ProviderConfig) (string, error) {
	if providerConfig == nil {
		return existingTokenURL, nil
	}

	// Prioritize the new AuthConfig field as the source of truth.
	if providerConfig.Spec.AuthConfig != nil && providerConfig.Spec.AuthConfig.TokenURL != "" {
		return providerConfig.Spec.AuthConfig.TokenURL, nil
	}

	// Fallback for older ProviderConfigs that don't have the AuthConfig field.
	if _, ok := providerConfig.Labels[tenantLabelKey]; !ok {
		// If no tenant label is set, assume providerConfig is for the cluster project.
		// This means we will use the existing standard token URL (/generateToken).
		return existingTokenURL, nil
	}

	return TokenURLFromProjectNumberAndName(existingTokenURL, strconv.FormatInt(providerConfig.Spec.ProjectNumber, 10), providerConfig.Name)
}

// TokenURLForTenantProject generates the token URL based on the tenant project.
// The constructed URL is of the form:
// projects/{CLUSTER_PROJECT_NUMBER}/locations/{CLUSTER_LOCATION}/clusters/{CLUSTER_NAME}/tenantProjects/{TENANT_PROJECT_NUMBER}:generateTenantProjectToken
func TokenURLForTenantProject(existingTokenURL string, tenantProjectNumber string) (string, error) {
	lastColonIndex := strings.LastIndex(existingTokenURL, ":")
	if lastColonIndex == -1 || strings.Count(existingTokenURL, ":") != 2 {
		return "", fmt.Errorf("invalid existing token URL format: %s, cannot extract base URL", existingTokenURL)
	}
	baseURL := existingTokenURL[:lastColonIndex]
	tokenURL := fmt.Sprintf("%s/tenantProjects/%s:generateTenantProjectToken", baseURL, tenantProjectNumber)
	return tokenURL, nil
}

// TokenURLFromProjectNumberAndName generates the token URL based on the project number and name.
func TokenURLFromProjectNumberAndName(existingTokenURL string, projectNumber, name string) (string, error) {
	location := extractLocationFromTokenURL(existingTokenURL)
	if location == "" {
		return "", fmt.Errorf("could not extract location from existing token URL: %s", existingTokenURL)
	}

	tokenURLParts := strings.SplitN(existingTokenURL, "/projects/", 2)
	if len(tokenURLParts) != 2 {
		return "", fmt.Errorf("invalid existing token URL format: %s, cannot extract base URL", existingTokenURL)
	}
	baseURL := tokenURLParts[0]

	// Format: {BASE_URL}/projects/{TENANT_PROJECT_NUMBER}/locations/{TENANT_LOCATION}/tenants/{TENANT_ID}:generateTenantToken
	formatString := "%s/projects/%s/locations/%s/tenants/%s:generateTenantToken"
	tokenURL := fmt.Sprintf(formatString, baseURL, projectNumber, location, name)
	return tokenURL, nil
}

// TokenBodyForProviderConfig updates the project number in the token body based on the ProviderConfig.
// It first checks for the new AuthConfig spec field. If present, it returns the TokenBody from there.
// As a fallback for backward compatibility, it checks for a tenant label to determine
// if the token body should be updated with the tenant's project number.
func TokenBodyForProviderConfig(existingTokenBody string, providerConfig *providerconfigv1.ProviderConfig) (string, error) {
	if providerConfig == nil {
		return existingTokenBody, nil
	}

	// Prioritize the new AuthConfig field as the source of truth.
	if providerConfig.Spec.AuthConfig != nil && providerConfig.Spec.AuthConfig.TokenBody != "" {
		return providerConfig.Spec.AuthConfig.TokenBody, nil
	}
	// Fallback for older ProviderConfigs that don't have the AuthConfig field.
	if _, ok := providerConfig.Labels[tenantLabelKey]; !ok {
		// If no tenant label is set, assume providerConfig is for the cluster project.
		// This means we will use the existing standard token body.
		return existingTokenBody, nil
	}

	return TokenBodyFromProjectNumber(existingTokenBody, strconv.FormatInt(providerConfig.Spec.ProjectNumber, 10))
}

// TokenBodyFromProjectNumber updates the project number in the token body based on the given project number.
// existingTokenBody is expected to be in the format of
// `"{\"projectNumber\":12345,\"clusterId\":\"example-cluster\"}"`.
func TokenBodyFromProjectNumber(existingTokenBody string, projectNumber string) (string, error) {
	// Check if the token body is a quoted JSON string
	// Quoted example: "{\"projectNumber\":12345,\"clusterId\":\"example-cluster\"}"
	// Non-quoted example: {"projectNumber":12345,"clusterId":"example-cluster"}
	jsonStr, isQuoted, err := unquoteJSON(existingTokenBody)
	if err != nil {
		return "", err
	}

	var bodyMap map[string]any

	if err := json.Unmarshal([]byte(jsonStr), &bodyMap); err != nil {
		return "", fmt.Errorf("error unmarshaling TokenBody: %v", err)
	}

	p, err := strconv.ParseInt(projectNumber, 10, 64)
	if err != nil {
		return "", fmt.Errorf("error converting projectNumber to int64: %v", err)
	}
	bodyMap["projectNumber"] = p

	newTokenBodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return "", fmt.Errorf("error marshaling TokenBody: %v", err)
	}

	if isQuoted {
		// Re-quote the JSON string if the original was quoted
		return strconv.Quote(string(newTokenBodyBytes)), nil
	}

	return string(newTokenBodyBytes), nil
}

// extractLocationFromTokenURL extracts the location from a GKE token URL.
// Example input: https://gkeauth.googleapis.com/v1/projects/654321/locations/us-central1/clusters/example-cluster:generateToken
// Returns: us-central1
func extractLocationFromTokenURL(tokenURL string) string {
	parts := strings.Split(tokenURL, "/")
	for i, part := range parts {
		if part == "locations" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
