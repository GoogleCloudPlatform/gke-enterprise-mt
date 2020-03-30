# GKE Enterprise Multi-Tenancy Cluster Example.

This example creates two private GKE clusters in a shared VPC following the
enterprise multi-tenant best practices as appropriate.

The VPC host project is created in the given network adminstrator folder, and
the cluster service projects are created in the given cluster adminstrator
folder.

G Suite features, such as authenticator security groups, are not enabled.

## Prerequisites

