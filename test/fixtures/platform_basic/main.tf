provider "random" {
  version = "~> 2.0"
}

module "example" {
  source = "../../../examples/platform_basic"

  organization_id = var.organization_id
  billing_id = var.billing_id
  parent_folder_id = var.folder_id
  domain = var.domain
}
