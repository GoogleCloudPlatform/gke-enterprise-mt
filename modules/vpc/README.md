# Shared VPC module

This modules creates a shared VPC consistent with the [Enterprise Multi-Tenancy Best Practices
Guide](https://cloud.google.com/kubernetes-engine/docs/best-practices/enterprise-multitenancy).

The `subnets` variable should be populated with subnets for each GKE cluster,
and the `host_project_id` should be the same as that used with the clusters.
