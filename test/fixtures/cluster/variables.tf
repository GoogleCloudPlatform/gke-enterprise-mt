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
