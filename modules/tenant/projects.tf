locals {
  cleaned_name = replace(lower(var.tenant_name), " ", "-")
}

// Random Id generator for unique project ID
resource "random_id" "random_proj_id_suffix" {
  byte_length = 2
}

#Module to create tenant service Project
module "project_tenant_service" {
  source                  = "terraform-google-modules/project-factory/google//modules/shared_vpc"
  version                 = "~> 3.3.0"
  random_project_id       = "false"
  name                    = format("%s-tenant-%s-%s", var.prefix, local.cleaned_name, random_id.random_proj_id_suffix.hex)
  project_id              = format("%s-tenant-%s-%s", var.prefix, local.cleaned_name, random_id.random_proj_id_suffix.hex)
  org_id                  = var.organization_id
  folder_id               = module.folder_tenant.id
  billing_account         = var.billing_account
  default_service_account = "keep"
  shared_vpc              = var.shared_vpc_project_id
  activate_apis           = ["compute.googleapis.com", "container.googleapis.com"]
  labels                  = { "tenant" : format("tenant-%s", local.cleaned_name) }
}
