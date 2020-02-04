variable "organization_id" {
  description = "The organization ID"
  type        = string
}

variable "billing_account" {
  description = "The ID of the billing account to associate the projects with"
  type        = string
}

variable "tenant_root_folder_id" {
  description = "The ID of the root tenant folder"
  type        = string
}

variable "tenant_name" {
  description = "An identifying name for the tenant"
  type        = string
}

variable "shared_vpc_project_id" {
  description = "Project id for shared VPC host project"
  type        = string
}

variable "prefix" {
  description = "The prefix used for projects and dataset ID/names"
  type        = string
}
