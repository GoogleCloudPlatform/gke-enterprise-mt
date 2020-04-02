# Shared VPC Private GKE Cluster module.

This modules creates GKE shared VPC private cluster consistent with the
[Enterprise Multi-Tenancy Best Practices
Guide](https://cloud.google.com/kubernetes-engine/docs/best-practices/enterprise-multitenancy).

There must be a [shared VPC network](../vpc/README.md) to connect to this
cluster. The `host_project_id` used here must be the same as for that network,
and the `ip_range*` variables must match the subnets used in the VPC
network. See [the cluster example](../../examples/cluster/README.md) for one
approach to using this.
