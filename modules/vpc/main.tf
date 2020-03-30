#Module to create Network and Enabling SVPC Host Project for Production
module "vpc_shared_net_prod" {
  source          = "terraform-google-modules/network/google"
  version         = "~> 1.2.0"
  project_id      = var.host_project_id
  network_name    = var.network_name
  shared_vpc_host = "true"
  subnets = [ for s in var.subnets :
    {
      subnet_name           = s.name
      subnet_ip             = s.ip
      subnet_region         = s.region
      subnet_private_access = "true"
    }
  ]
  secondary_ranges = { for s in var.subnets :
    s.name => [
      {
        range_name    = format("%s-pod-secondary", s.name)
        ip_cidr_range = s.pod
      },
      {
        range_name    = format("%s-services-secondary", s.name)
        ip_cidr_range = s.svc
      }
    ]
  }
}
