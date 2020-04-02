provider "random" {
  version = "~> 2.0"
}

module "example" {
  source = "../../../examples/cluster"

  organization_id = var.organization_id
  billing_account = var.billing_account
  network_folder_id = var.root_folder_id
  cluster_folder_id = var.root_folder_id
  prefix = "etst"
}
