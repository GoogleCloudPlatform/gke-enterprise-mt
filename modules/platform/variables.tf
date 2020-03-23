variable "organization_id" {
  description = "The organization ID"
  type        = string
}

variable "prefix" {
  description = "The prefix used for projects and dataset ID/names"
  type        = string
}

variable "cluster_host_project_id" {
  description = "The project id hosting the GKE cluster"
  type        = string
}

variable "cluster_service_project_id" {
  description = "The service project id for the GKE cluster"
  type        = string
}

variable "cluster_service_project_number" {
  description = "The service project number for the GKE cluster. Note this is not the project *id*"
  type        = string
}

variable "tenant_host_project_id" {
  description = "The project id used to host tenants"
  type        = string
}

variable "org_admin_group" {
  description = "The gsuite org admin group"
  type        = string
}

variable "folder_admin_group" {
  description = "The gsuite folder admin group"
  type        = string
}

variable "network_admin_group" {
  description = "The gsuite network admin group"
  type        = string
}

variable "security_admin_group" {
  description = "The gsuite network admin group"
  type        = string
}

variable "gke_security_group" {
  description = "The gsuite gke security group"
  type        = string
}

variable "auditor_group" {
  description = "The gsuite auditor group"
  type        = string
}

variable "cluster_admin_group" {
  description = "The gsuite cluster admin group"
  type        = string
}

variable "tenant_admin_group" {
  description = "The gsuite tenant admin group"
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

variable "enable_pod_security_policy" {
  description = "Enable pod security policy (PSP) for the cluster. Because PSP is not compatible with all standard containers, the default is off. We recommend that this be set true if possble, or some alternative security policy mechanism used."
  default     = false
}
