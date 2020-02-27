terraform {
  required_version = ">= 0.12"
}

provider "google" {
  version = "~> 2.19.0"
}

provider "google-beta" {
  version = "~> 3.1.0"
}

provider "template" {
  version = "~> 2.1"
}

module "platform_basic" {
  source = "../../modules/platform"

  organization_id = var.organization_id
  domain = var.domain
  cluster_host_project_id = module.project_cluster_host.project_id
  cluster_service_project_id = module.project_cluster_service.project_id
  cluster_service_project_number = module.project_cluster_service.project_number
  tenant_host_project_id = module.project_tenant_host.project_id
  region = var.region
  prefix = var.prefix
}
