provider "random" {
  version = "~> 2.0"
}

module "example" {
  source = "../../../examples/platform_basic"

  organization_id = var.organization_id
  billing_account = var.billing_account
  parent_folder_id = replace(var.root_folder, "folder/", "")
  prefix = "mt-test"
  domain = var.domain
}
