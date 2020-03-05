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

provider "gsuite" {
  impersonated_user_email = var.gsuite_admin_account
  credentials = file(var.creds_path)

  oauth_scopes = [
    "https://www.googleapis.com/auth/admin.directory.group",
    "https://www.googleapis.com/auth/admin.directory.user",
  ]

  version = "~> 0.1.12"
}

module "platform" {
  source = "../../modules/platform"

  organization_id = var.organization_id
  cluster_host_project_id = module.project_cluster_host.project_id
  cluster_service_project_id = module.project_cluster_service.project_id
  cluster_service_project_number = module.project_cluster_service.project_number
  tenant_host_project_id = module.project_tenant_host.project_id
  region = var.region
  prefix = var.prefix

  org_admin_group = gsuite_group.org-admin.email
  folder_admin_group = gsuite_group.folder-admin.email
  network_admin_group = gsuite_group.network-admin.email
  security_admin_group = gsuite_group.security-admin.email
  gke_security_group = gsuite_group.gke-security.email
  auditor_group = gsuite_group.auditor.email
  cluster_admin_group = gsuite_group.cluster-admin.email
  tenant_admin_group = gsuite_group.tenant-admin.email
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
