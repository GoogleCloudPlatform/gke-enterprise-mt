#Module to create Network and Enabling SVPC Host Project for Production
module "vpc_shared_net_prod" {
  source          = "terraform-google-modules/network/google"
  version         = "~> 1.2.0"
  project_id      = var.cluster_host_project_id
  network_name    = "shared-net-prod"
  shared_vpc_host = "true"
  subnets = [
    {
      subnet_name           = "prod-subnet"
      subnet_ip             = "10.49.0.0/16"
      subnet_region         = var.region
      subnet_private_access = "true"
    }
  ]
  secondary_ranges = {
    "prod-subnet" = [
      {
        range_name    = "prod-subnet-pod-secondary"
        ip_cidr_range = "10.48.0.0/16"
      },
      {
        range_name    = "prod-subnet-services-secondary"
        ip_cidr_range = "10.50.0.0/16"
      }
    ]
  }
}

#Module to create Network and Enabling SVPC Host for Tenant Host Project
module "vpc_shared_net_tenant" {
  source          = "terraform-google-modules/network/google"
  version         = "~> 1.2.0"
  project_id      = var.tenant_host_project_id
  network_name    = "shared-net-tenant"
  shared_vpc_host = "true"
  subnets = [
    {
      subnet_name           = "tenant-subnet"
      subnet_ip             = "10.0.0.0/18"
      subnet_region         = var.region
      subnet_private_access = "true"
    }
  ]
  secondary_ranges = {
    "tenant-subnet" = []
  }
}

# Module to create vpc peering between cluster and tenant vpc
module "network_network-peering" {
  source        = "terraform-google-modules/network/google//modules/network-peering"
  version       = ">= 2.0"
  prefix        = var.prefix
  local_network = module.vpc_shared_net_prod.network_self_link
  peer_network  = module.vpc_shared_net_tenant.network_self_link
}
