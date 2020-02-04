variable "organization_id" {
  description = "The organization ID"
  type        = string
}

variable "billing_account" {
  description = "The ID of the billing account to associate the projects with"
  type        = string
}
variable "folder_id" {
  description = "The ID of the parent folder"
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

