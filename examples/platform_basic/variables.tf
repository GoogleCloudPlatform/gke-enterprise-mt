variable "organization_id" {
  description = "The organization ID"
  type        = string
}

variable "billing_account" {
  description = "The ID of the billing account to associate the projects with"
  type        = string
}

variable "region" {
  description = "region"
  type        = string
  default     = "us-central1"
}

variable "prefix" {
  description = "The prefix used for projects and dataset ID/names"
  type        = string
}

variable "domain" {
  description = "The domain name"
  type        = string
}

variable "gsuite_admin_account" {
  description = "The account email of an admin for the domain, above"
  type        = string
}

variable "parent_folder_id" {
  description = "The ID of the parent folder"
  type        = string
}

variable "creds_path" {
  description = "Path to terraform service account credentials file"
  type        = string
}
