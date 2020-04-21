provider "random" {
  version = "~> 2.0"
}

module "example" {
  source = "../../../examples/cluster"

  organization_id = var.organization_id
  billing_account = var.billing_account
  network_folder_id = var.root_folder_id
  cluster_folder_id = var.root_folder_id
  # Since we don't know where we might connect to the cluster from for testing purposes,
  # we open the MAN to the world.
  master_x_authorized_network = "0.0.0.0/32"
  master_y_authorized_network = "0.0.0.0/32"
  prefix = "etst"
}
