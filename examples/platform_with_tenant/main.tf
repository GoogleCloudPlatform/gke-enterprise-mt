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

module "platform" {
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

module "tenant" {
  source = "../../modules/tenant"

  organization_id = var.organization_id
  billing_account = var.billing_account
  tenant_root_folder_id = module.folder_tenants.id
  prefix = var.prefix
  tenant_name = "Dev"
  shared_vpc_project_id = module.project_cluster_host.project_id
}
