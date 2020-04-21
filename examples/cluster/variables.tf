variable "organization_id" {
  description = "The organization id associated with these resources"
  type        = string
}

variable "billing_account" {
  description = "The billing account associated with projects"
  type        = string
}

variable "network_folder_id" {
  description = "The folder id to hold the network projects"
  type        = string
}

variable "cluster_folder_id" {
  description = "The folder id to hold the cluster projects"
  type        = string
}

variable "prefix" {
  description = "The prefix used for projects and dataset ID/names"
  type        = string
}

variable "master_x_authorized_network" {
  description = "The CIDR range for the network that will be authorized to access cluster x"
  type        = string
}

variable "master_y_authorized_network" {
  description = "The CIDR range for the network that will be authorized to access cluster x"
  type        = string
}

variable "region_x" {
  description = "The region used for cluster X"
  default     = "us-central1"
}

variable "region_x_zones" {
  description = "The zones to use with region_x"
  default     = ["us-central1-b", "us-central1-c", "us-central1-f"]
}

variable "region_y" {
  description = "The region used for cluster Y"
  default     = "us-west2"
}

variable "region_y_zones" {
  description = "The zones to use with region_y"
  default     = ["us-west2-a", "us-west2-b", "us-west2-c"]
}
