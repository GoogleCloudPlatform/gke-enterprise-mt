provider "random" {
  version = "~> 2.0"
}

provider "gsuite" {
  impersonated_user_email = var.gsuite_admin_account
  credentials = file(var.creds_path)

  oauth_scopes = [
    "https://www.googleapis.com/auth/admin.directory.group",
    "https://www.googleapis.com/auth/admin.directory.group.member",
  ]

  version = "~> 0.1.12"
}

module "example" {
  source = "../../../examples/platform_basic"

  organization_id = var.organization_id
  billing_account = var.billing_account
  parent_folder_id = var.root_folder_id
  prefix = "mt-0"
  domain = var.domain
  gsuite_admin_account = var.gsuite_admin_account
  creds_path = var.creds_path
}
