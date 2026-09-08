package filteredinformerv2

import (
	"fmt"

	"google3/third_party/kubernetes_apis/k8s_io/apimachinery/pkg/api/meta/meta"
	"k8s.io/client-go/tools/cache"
)

const (
	providerConfigLabel     = "tenancy.gke.io/provider-config"
	providerConfigIndexName = "provider-config"
)

// ProviderConfigIndexFunc is an index function that indexes based on provider config.
func ProviderConfigIndexFunc(obj any) ([]string, error) {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return nil, fmt.Errorf("object has no meta: %w", err)
	}
	labels := metaObj.GetLabels()
	if providerConfig, ok := labels[providerConfigLabel]; ok {
		return []string{providerConfig}, nil
	}
	return nil, nil
}

// isObjectInProviderConfig checks if an object belongs to a specific provider config.
func isObjectInProviderConfig(obj any, providerConfigName string) bool {
	if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = d.Obj
	}
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return false
	}
	return metaObj.GetLabels()[providerConfigLabel] == providerConfigName
}

// providerConfigFilteredList filters a list of objects by provider config name.
func providerConfigFilteredList(items []any, providerConfigName string) []any {
	filtered := make([]any, 0, len(items))
	for _, item := range items {
		if isObjectInProviderConfig(item, providerConfigName) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
