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

module "platform_basic" {
  source = "../../modules/platform"

  organization_id = var.organization_id
  billing_account = var.billing_account
  region = var.region
  prefix = var.prefix
  domain = var.domain
  parent_folder_id = var.parent_folder_id
}
