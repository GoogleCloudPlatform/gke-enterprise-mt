// These outputs are used to test the module with kitchen-terraform
// They do not need to be included in real-world uses of this module

output "project_cluster_service_project_id" {
  value = module.project_cluster_service.project_id
}

output "project_cluster_host_project_id" {
  value = module.project_cluster_host.project_id
}

output "vpc_shared_net_prod" {
  value = module.platform_basic.vpc_shared_net_prod
}

output "folder_prod_id" {
  value = module.folder_prod.id
}

output "location" {
  value = var.region
}

output "cluster_name" {
  value = module.platform_basic.cluster_prod.name
}
