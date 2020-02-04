# terraform-google-enterprise-mt

This module was generated from
[terraform-google-module-template](https://github.com/terraform-google-modules/terraform-google-module-template/). As
the module develops, this README will be updated.

See the [Enterprise Multi-Tenancy Guide](https://cloud.google.com/kubernetes-engine/docs/best-practices/enterprise-multitenancy) for more details.

## Usage

See the [platform module README](module/platform/README.md) for usage.

Functional examples are included in the
[examples](./examples/) directory.

## Requirements

These sections describe general requirements for using CFT modules. See the
modules for any specific requirements.

### Software

The following dependencies must be available:

- [Terraform][terraform] v0.12
- [Terraform Provider for GCP][terraform-provider-gcp] plugin v2.0

### Service Account

A service account with the following roles must be used to provision
the resources of this module:

- Storage Admin: `roles/storage.admin`

The [Project Factory module][project-factory-module] and the
[IAM module][iam-module] may be used in combination to provision a
service account with the necessary roles applied.

### APIs

A project with the following APIs enabled must be used to host the
resources of this module:

- Google Cloud Storage JSON API: `storage-api.googleapis.com`

The [Project Factory module][project-factory-module] can be used to
provision a project with the necessary APIs enabled.

## Contributing

Refer to the [contribution guidelines](./CONTRIBUTING.md) for
information on contributing to this module.

[iam-module]: https://registry.terraform.io/modules/terraform-google-modules/iam/google
[project-factory-module]: https://registry.terraform.io/modules/terraform-google-modules/project-factory/google
[terraform-provider-gcp]: https://www.terraform.io/docs/providers/google/index.html
[terraform]: https://www.terraform.io/downloads.html
