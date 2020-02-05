variable "organization_id" {
  description = "The organization ID"
  type        = string
}

variable "billing_account" {
  description = "The ID of the billing account to associate the projects with"
  type        = string
}

variable "root_folder" {
  description = "The name of the root folder to run the test under (from test/setup/)"
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

