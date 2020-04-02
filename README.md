# terraform-google-enterprise-mt

See the [Enterprise Multi-Tenancy Best Practices
Guide](https://cloud.google.com/kubernetes-engine/docs/best-practices/enterprise-multitenancy)
for an overview of the philosophy of this module.

This module is at an alpha level of development. It is being actively worked on,
and refactorings will be frequest. Please start an issue for any friction you
have or suggestions you have for better organization.

As such, there are no tagged releases yet. Expect backward-incompatible and
breaking changes to be frequent. Fork this repo if you need some stability.

## Usage

The [cluster](examples/cluster/README.md) example deploys a shared VPC with two
GKE private cluster with a bastion host as described in the best practices
guide. It creates separate host and service projects as described in the best
practices guide in provided GCP folders.

The example is based on the [cluster](modules/cluster/README.md) and
[vpc](modules/vpc/README.md) modules which can be mixed in with existing fleets.

This blueprint does not set up organization folder structure or G Suite groups,
beyond providing a hook for an existing security group. We have found that
configuration depends quite a bit on existing account structure and so is
difficult to provide canned scripts.

## Contributing

This module was generated from
[terraform-google-module-template](https://github.com/terraform-google-modules/terraform-google-module-template/)
and retains for the most part the standard directory and testing structure
familiar to other Cloud Foundation Toolkit modules.

Refer to the [contribution guidelines](./CONTRIBUTING.md) for
information on contributing to this module.
