# terraform-google-enterprise-mt

See the [Enterprise Multi-Tenancy Best Practices
Guide](https://cloud.google.com/kubernetes-engine/docs/best-practices/enterprise-multitenancy)
for an overview of the philosophy of this module.

## Usage

The [platform](modules/platform/README.md) terraform module deploys a shared VPC GKE
private cluster with a bastion host as described in the best practices guide,
including setting up service accounts. It assumes that host and service projects
have already been set up, as well as the appropriate GCP folder structure.

The [tenant](modules/tenant/README.md) modules provisions a tenant, which
currently just means provisioning a GCP folder and tenant project.

These two modules are suitable for mixing in to an existing deployment, for
example if you already have an existing GCP folder structure.

Functional examples are included in the [examples](./examples/) directory. In
particular, these examples assume only a GCP organization and root folder in
which to create the setup as described in the best practices guide. They are
suitable for demos and probably could not be directly used for a real deployment.

## Contributing

This module was generated from
[terraform-google-module-template](https://github.com/terraform-google-modules/terraform-google-module-template/)
and retains for the most part the standard directory and testing structure
familiar to other Cloud Foundation Toolkit modules.

Refer to the [contribution guidelines](./CONTRIBUTING.md) for
information on contributing to this module.
