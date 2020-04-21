provider "google" {
  version = "~> 3.15.0"
}

provider "google-beta" {
  version = "~> 3.15.0"
}

resource "random_id" "suffix" {
  byte_length = 2
}

locals {
  creds_path           = "terraform-hmt-2901.json"

  host_project_id      = format("%s-host-%s", var.prefix, random_id.suffix.hex)
  service_x_project_id = format("%s-service-x-%s", var.prefix, random_id.suffix.hex)
  service_y_project_id = format("%s-service-y-%s", var.prefix, random_id.suffix.hex)

  shared_vpc_name      = format("%s-network", var.prefix)

  subnet_x = {
    name   = format("%s-subnet-x", var.prefix)
    ip     = "10.47.0.0/16"
    region = var.region_x
    pod    = "10.48.0.0/16"
    svc    = "10.49.0.0/16"
  }
  subnet_y = {
    name   = format("%s-subnet-y", var.prefix)
    ip     = "10.57.0.0/16"
    region = var.region_y
    pod    = "10.58.0.0/16"
    svc    = "10.59.0.0/16"
  }
  subnets      = [ local.subnet_x, local.subnet_y ]
  master_x_ips = "10.31.0.0/28"
  master_y_ips = "10.32.0.0/28"
}

module "host_project" {
  source                  = "terraform-google-modules/project-factory/google"
  version                 = "~> 7.0.0"
  name                    = local.host_project_id
  org_id                  = var.organization_id
  folder_id               = var.network_folder_id
  billing_account         = var.billing_account
  default_service_account = "keep"
  activate_apis           = ["compute.googleapis.com", "container.googleapis.com"]
}

module "service_x_project" {
  source                  = "terraform-google-modules/project-factory/google//modules/shared_vpc"
  version                 = "~> 7.0.0"
  name                    = local.service_x_project_id
  org_id                  = var.organization_id
  folder_id               = var.cluster_folder_id
  billing_account         = var.billing_account
  default_service_account = "keep"
  shared_vpc              = module.host_project.project_id
  activate_apis           = ["compute.googleapis.com", "container.googleapis.com"]
}

module "service_y_project" {
  source                  = "terraform-google-modules/project-factory/google//modules/shared_vpc"
  version                 = "~> 7.0.0"
  name                    = local.service_y_project_id
  org_id                  = var.organization_id
  folder_id               = var.cluster_folder_id
  billing_account         = var.billing_account
  default_service_account = "keep"
  shared_vpc              = module.host_project.project_id
  activate_apis           = ["compute.googleapis.com", "container.googleapis.com"]
}

module "shared_vpc" {
  source = "../../modules/vpc"

  prefix          = var.prefix
  network_name    = local.shared_vpc_name
  subnets         = local.subnets
  host_project_id = module.host_project.project_id
}

module "gke_x" {
  source = "../../modules/cluster/"
  
  prefix                 = var.prefix
  host_project_id        = module.host_project.project_id
  service_project_id     = module.service_x_project.project_id
  service_project_number = module.service_x_project.project_number
  gke_security_group     = null
  region                 = var.region_x
  zones                  = var.region_x_zones
  cluster_name           = "cluster-x"

  network_name           = module.shared_vpc.network_name
  subnet_name            = local.subnet_x.name
  master_ipv4_cidr_block = local.master_x_ips
  master_authorized_networks = var.master_x_authorized_network
  ip_range_pods          = module.shared_vpc.secondary_ranges[local.subnet_x.name].pod
  ip_range_services      = module.shared_vpc.secondary_ranges[local.subnet_x.name].svc
}  

module "gke_y" {
  source = "../../modules/cluster/"
  
  prefix                 = var.prefix
  host_project_id        = module.host_project.project_id
  service_project_id     = module.service_y_project.project_id
  service_project_number = module.service_y_project.project_number
  gke_security_group     = null
  region                 = var.region_y
  zones                  = var.region_y_zones
  cluster_name           = "cluster-y"

  network_name           = local.shared_vpc_name
  subnet_name            = local.subnet_y.name
  master_ipv4_cidr_block = local.master_y_ips
  master_authorized_networks = var.master_y_authorized_network
  ip_range_pods          = module.shared_vpc.secondary_ranges[local.subnet_y.name].pod
  ip_range_services      = module.shared_vpc.secondary_ranges[local.subnet_y.name].svc
}  
