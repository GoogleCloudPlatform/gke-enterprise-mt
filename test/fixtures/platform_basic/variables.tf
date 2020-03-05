variable "organization_id" {
  description = "The organization ID"
  type        = string
}

variable "billing_account" {
  description = "The ID of the billing account to associate the projects with"
  type        = string
}

variable "root_folder_id" {
  description = "The id of the root folder to run the test under (from test/setup/)"
  type        = string
}

variable "region" {
  description = "region"
  type        = string
  default     = "us-central1"
}

variable "domain" {
  description = "The domain name registered with organization_id"
  type        = string
}

variable "gsuite_admin_account" {
  description = "The account email of an admin for the domain, above"
  type        = string
}

variable "creds_path" {
  description = "Path to terraform service account credentials file"
  type        = string
}
