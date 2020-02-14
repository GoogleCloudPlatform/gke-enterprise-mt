output "project_hmt_prod_service_project_id" {
  value = module.example.project_hmt_prod_service_project_id
}

output "project_hmt_prod_host_project_id" {
  value = module.example.project_hmt_prod_host_project_id
}

output "vpc_shared_net_prod_name" {
  value = module.example.vpc_shared_net_prod.name
}

output "vpc_shared_net_prod_subnet_names" {
  value = [for s in module.example.vpc_shared_net_prod.subnets: s.subnet_name]
}

output "folder_prod_id" {
  value = module.example.folder_prod_id
}

output "region" {
  value = var.region
}

output "cluster_name" {
  value = module.example.cluster_name
}
