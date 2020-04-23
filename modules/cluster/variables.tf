variable "prefix" {
  description = "The prefix used for projects and dataset ID/names"
  type        = string
}

variable "host_project_id" {
  description = "The project id hosting the GKE cluster"
  type        = string
}

variable "service_project_id" {
  description = "The service project id for the GKE cluster"
  type        = string
}

variable "service_project_number" {
  description = "The service project number for the GKE cluster. Note this is not the project *id*"
  type        = string
}

variable "gke_security_group" {
  description = "The gsuite gke security group"
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

variable "cluster_name" {
  description = "The name of the prod GKE cluster"
  type        = string
}

variable "network_name" {
  description = "Shared VPC network name"
  type        = string
}

variable "subnet_name" {
  description = "Shared VPC subnetwork name"
  type        = string
}

variable "ip_range_pods" {
  description = "GKE pods secondary ip range name"
  type        = string
}

variable "ip_range_services" {
  description = "GKE services secondary ip range name"
  type        = string
}

variable "master_ipv4_cidr_block" {
  description = "GKE master CIDR block. Must be /28"
  type        = string
}

variable "master_authorized_networks" {
  description = "Master access restricted to this network"
  type        = string
}

variable "max_nodes" {
  description = "Maximum number of nodes to autoscale to"
  default     = 5
}

variable "enable_intranode_visibility" {
  description = "Enable intranode visibility (usually for MCSD)"
  type        = bool
  default     = false
}

variable "dataset_labels" {
  description = "Usage metering bigquery dataset labels"
  type        = map(string)
  default     = {env = "prod"}
}

variable "use_private_endpoints" {
  description = "Use private master endpoints"
  default     = true
}

variable "enable_pod_security_policy" {
  description = "Enable pod security policy (PSP) for the cluster. Because PSP is not compatible with all standard containers, the default is off. We recommend that this be set true if possble, or some alternative security policy mechanism used."
  default     = false
}
