# Folder Creation
#
# Rather than using a single subfolder module with multiple names, the subfolders
# are repeated so that they can be referred to.

module "folder_cluster_team" {
  source  = "terraform-google-modules/folders/google"
  version = "~>2.0.0"
  parent  = "folders/${var.parent_folder_id}"
  names   = ["Cluster Team"]
}

module "folder_prod" {
  source  = "terraform-google-modules/folders/google"
  version = "~>2.0.0"
  parent  = module.folder_cluster_team.id
  names   = ["Prod"]
}

module "folder_staging" {
  source  = "terraform-google-modules/folders/google"
  version = "~>2.0.0"
  parent  = module.folder_cluster_team.id
  names   = ["Staging"]
}

module "folder_dev" {
  source  = "terraform-google-modules/folders/google"
  version = "~>2.0.0"
  parent  = module.folder_cluster_team.id
  names   = ["Dev"]
}

module "folder_network_team" {
  source  = "terraform-google-modules/folders/google"
  version = "~>2.0.0"
  parent  = "folders/${var.parent_folder_id}"
  names   = ["Network Team"]
}

module "folder_tenantnet" {
  source  = "terraform-google-modules/folders/google"
  version = "~>2.0.0"
  parent  = module.folder_network_team.id
  names   = ["TenantNet"]
}

module "folder_clusternet" {
  source  = "terraform-google-modules/folders/google"
  version = "~>2.0.0"
  parent  = module.folder_network_team.id
  names   = ["ClusterNet"]
}

module "folder_tenants" {
  source  = "terraform-google-modules/folders/google"
  version = "~>2.0.0"
  parent  = "folders/${var.parent_folder_id}"
  names   = ["Tenants"]
}
