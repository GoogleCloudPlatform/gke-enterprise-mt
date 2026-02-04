# GKE Enterprise Multi-Tenancy Framework

This repository implements a controller framework for managing multi-tenant Kubernetes controllers in GKE Enterprise.

## Overview

The core of this project is a "meta-controller" or **Manager** that dynamically starts and stops sets of controllers for each tenant. Tenants are defined by `ProviderConfig` resources. This approach ensures strict isolation and lifecycle management for tenant-specific logic.

## Architecture

### ProviderConfig
The `ProviderConfig` Custom Resource Definition (CRD) acts as the source of truth for a tenant's configuration. It controls the lifecycle of tenant-specific controllers.

### Framework Manager
The Manager (`pkg/framework/manager.go`) watches `ProviderConfig` objects.
- **On Add/Update**: It spins up a new set of controllers (e.g., NodeController, IPAMController) dedicated to that tenant.
- **On Delete**: It ensures all tenant-specific controllers are stopped and cleans up resources (via Finalizers) before allowing the `ProviderConfig` to be deleted.
- **Idempotency**: The manager ensures that repeated events do not trigger duplicate controller startups.

### Isolation
Controllers are "scoped" to their tenant to ensure they only process resources (like Nodes) belonging to that tenant. This is achieved through:
- **Filtered Informers**: Ensuring controllers only see objects matching specific labels or fields.
- **Scoped Clients**: Restricting API access where possible.

## Directory Structure

| Directory | Description |
|-----------|-------------|
| `apis/` | Kubernetes API definitions (CRDs), specifically `ProviderConfig`. |
| `pkg/framework/` | Core logic for the controller manager and lifecycle coordination. |
| `pkg/providerconfig/` | Client sets, listers, and informers for the custom resources. |
| `pkg/utils/` | Shared utilities for workqueues and common patterns. |
| `pkg/finalizer/` | Helper logic for managing Kubernetes finalizers. |

## Development

### Prerequisites
- Go 1.25.5+
- Kubernetes environment (or test setup)

### Build
To build the project:
```bash
make build
```

### Test
To run unit and race detection tests:
```bash
make test
```

### Utilities
- `make fmt`: Format code
- `make tidy`: Tidy Go modules
- `make vet`: Run `go vet`
