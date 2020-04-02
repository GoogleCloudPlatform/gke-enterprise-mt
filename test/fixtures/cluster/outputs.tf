output "host_project_id" {
  value = module.example.host_project.project_id
}

output "service_x_project_id" {
  value = module.example.service_x_project.project_id
}

output "cluster_x_name" {
  value = module.example.cluster_x.name
}

output "region_x" {
  value = module.example.region_x
}

output "service_y_project_id" {
  value = module.example.service_y_project.project_id
}

output "cluster_y_name" {
  value = module.example.cluster_y.name
}

output "region_y" {
  value = module.example.region_y
}

output "shared_vpc_name" {
  value = module.example.shared_vpc_name
}

output "subnets_names" {
  value = module.example.subnets_names
}
