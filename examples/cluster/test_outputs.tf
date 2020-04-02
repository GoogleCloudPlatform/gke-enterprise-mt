output "host_project" {
  value = module.host_project
}

output "service_x_project" {
  value = module.service_x_project
}

output "cluster_x" {
  value = module.gke_x.cluster
}

output "service_y_project" {
  value = module.service_y_project
}

output "cluster_y" {
  value = module.gke_y.cluster
}

output "region_x" {
  value = var.region_x
}

output "region_y" {
  value = var.region_y
}

output "shared_vpc_name" {
  value = module.shared_vpc.network_name
}

output "subnets_names" {
  value = module.shared_vpc.subnets_names
}
