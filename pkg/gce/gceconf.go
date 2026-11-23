package gce

import (
	"fmt"
	"strings"

	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
)

// ConfCreator is an interface for creating a GCE config for a given provider config.
type ConfCreator interface {
	GCEConfForProviderConfig(providerConfig *providerconfigv1.ProviderConfig) (*Config, error)
}

// DefaultConfCreator creates a GCE config for a given provider config using a default strategy.
// For Tenant Projects, it will use /generateTenantToken.
// For a Cluster Project, it will use /generateToken.
type DefaultConfCreator struct {
	defaultConfig *Config
}

// NewDefaultConfCreator creates a new DefaultConfCreator.
func NewDefaultConfCreator(defaultConfig *Config) *DefaultConfCreator {
	return &DefaultConfCreator{
		defaultConfig: defaultConfig,
	}
}

// GCEConfForProviderConfig returns a new GCE config for the given project.
// If providerConfig is nil, it returns the default config associated with the cluster's project.
// It modifies the default configuration when a providerConfig is provided.
func (g *DefaultConfCreator) GCEConfForProviderConfig(providerConfig *providerconfigv1.ProviderConfig) (*Config, error) {
	if g.defaultConfig == nil {
		return nil, fmt.Errorf("defaultConfig is nil")
	}

	// Create a new config based on the default to avoid modifying the shared original.
	config := g.defaultConfig.DeepCopy()

	if providerConfig == nil {
		return config, nil
	}

	config.Global.ProjectID = providerConfig.Spec.ProjectID

	var err error
	config.Global.TokenURL, err = TokenURLForProviderConfig(config.Global.TokenURL, providerConfig)
	if err != nil {
		return nil, fmt.Errorf("error generating token URL: %w", err)
	}

	// Update TokenBody
	config.Global.TokenBodyRaw, err = TokenBodyForProviderConfig(config.Global.TokenBodyRaw, providerConfig)
	if err != nil {
		return nil, fmt.Errorf("error updating TokenBody: %w", err)
	}
	if config.Global.TokenBodyRaw != "" {
		config.Global.TokenBody, err = parseTokenBody(config.Global.TokenBodyRaw)
		if err != nil {
			return nil, err
		}
	} else if config.Global.TokenBody == nil {
		config.Global.TokenBody = &TokenBody{}
	}
	if config.Global.TokenBody.ProjectNumber == 0 {
		config.Global.TokenBody.ProjectNumber = int(providerConfig.Spec.ProjectNumber)
	}

	// Update NetworkName to be the last part of the network path
	if providerConfig.Spec.NetworkConfig.Network != "" {
		networkParts := strings.Split(providerConfig.Spec.NetworkConfig.Network, "/")
		config.Global.NetworkName = networkParts[len(networkParts)-1]
	}

	// Update NetworkURL (subnetwork) to be the last part of the subnetwork path
	if providerConfig.Spec.NetworkConfig.SubnetInfo.Subnetwork != "" {
		subnetworkParts := strings.Split(providerConfig.Spec.NetworkConfig.SubnetInfo.Subnetwork, "/")
		config.Global.NetworkURL = subnetworkParts[len(subnetworkParts)-1]
	}

	config.Global.NetworkProjectID = providerConfig.Spec.ProjectID

	return config, nil
}
