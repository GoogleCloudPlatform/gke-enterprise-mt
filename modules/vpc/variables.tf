variable "prefix" {
  description = "The prefix used for projects and dataset ID/names"
  type        = string
}

variable "network_name" {
  description = "The Shared VPC name"
  type        = string
}

variable "subnets" {
  description = "List of subnets"
  type = list(object({
    name   = string
    ip     = string  // subnet CIDR
    region = string
    pod    = string  // pod secondary network CIDR
    svc    = string  // service secondary network CIDR
  }))
}

variable "host_project_id" {
  description = "The project id hosting the GKE cluster"
  type        = string
}
