module "folder_tenant" {
  source  = "terraform-google-modules/folders/google"
  version = "~>2.0.0"
  parent  = var.tenant_root_folder_id
  names   = [format("Tenant %s", var.tenant_name)]
}
