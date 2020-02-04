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

variable "zones" {
  type        = list(string)
  default     = ["us-central1-a", "us-central1-b", "us-central1-f"]
}

variable "prefix" {
  description = "The prefix used for projects and dataset ID/names"
  type        = string
}

variable "domain" {
  description = "The domain name"
  type        = string
}

variable "parent_folder_id" {
  description = "The ID of the parent folder"
  type        = string
}

variable "prod_cluster_name" {
  description = "The name of the prod GKE cluster"
  type        = string
  default     = "prod"
}

variable "master_ipv4_cidr_block" {
  description = "GKE master CIDR block"
  type        = string
  default     = "10.51.0.0/28"
}

variable "master_authorized_networks_cidr" {
  description = "GKE master authorized network CIDR block"
  type        = string
  default     = "10.0.0.0/18"
}

variable "ip_range_pods" {
  description = "GKE pods secondary ip range name"
  type        = string
  default     = "prod-subnet-pod-secondary"
}

variable "ip_range_services" {
  description = "GKE services secondary ip range name"
  type        = string
  default     = "prod-subnet-services-secondary"
}
