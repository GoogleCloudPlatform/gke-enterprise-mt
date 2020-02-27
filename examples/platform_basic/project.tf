// Random Id generators for unique project ID
resource "random_id" "random_prod_cluster_suffix" {
  byte_length = 2
}

resource "random_id" "random_prod_service_suffix" {
  byte_length = 2
}

resource "random_id" "random_tenant_cluster_suffix" {
  byte_length = 2
}

# Create the cluster host project.
module "project_cluster_host" {
  source                  = "terraform-google-modules/project-factory/google"
  version                 = "~> 7.0.0"
  random_project_id       = "false"
  name                    = format("%s-prod-host-%s", var.prefix, random_id.random_prod_cluster_suffix.hex)
  org_id                  = var.organization_id
  folder_id               = module.folder_clusternet.id
  billing_account         = var.billing_account
  default_service_account = "keep"
  activate_apis           = ["compute.googleapis.com", "container.googleapis.com"]
}

# Create the service project for the cluster, for VPC and IAM.
module "project_cluster_service" {
  source                  = "terraform-google-modules/project-factory/google//modules/shared_vpc"
  version                 = "~> 7.0.0"
  random_project_id       = "false"
  name                    = format("%s-prod-service-%s", var.prefix, random_id.random_prod_service_suffix.hex)
  project_id              = format("%s-prod-service-%s", var.prefix, random_id.random_prod_service_suffix.hex)
  org_id                  = var.organization_id
  folder_id               = module.folder_prod.id
  billing_account         = var.billing_account
  default_service_account = "keep"
  shared_vpc              = module.project_cluster_host.project_id
  activate_apis           = ["compute.googleapis.com", "container.googleapis.com"]
  labels                  = { "env" : "prod" }
}

# Create the tenant host project.
module "project_tenant_host" {
  source                  = "terraform-google-modules/project-factory/google"
  version                 = "~> 7.0.0"
  random_project_id       = "false"
  name                    = format("%s-tenant-host-%s", var.prefix, random_id.random_tenant_cluster_suffix.hex)
  project_id              = format("%s-tenant-host-%s", var.prefix, random_id.random_tenant_cluster_suffix.hex)
  org_id                  = var.organization_id
  folder_id               = module.folder_tenantnet.id
  billing_account         = var.billing_account
  default_service_account = "keep"
  activate_apis           = ["compute.googleapis.com"]
}
