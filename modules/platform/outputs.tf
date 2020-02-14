output "project_hmt_prod_service_project_id" {
  value = module.project_hmt_prod_cluster_service.project_id
}

output "project_hmt_prod_host_project_id" {
  value = module.project_hmt_prod_cluster_host_prod.project_id
}

output "vpc_shared_net_prod" {
  value = module.vpc_shared_net_prod
}

output "folder_prod_id" {
  value = module.folder_prod.id
}

output "folder_tenants_id" {
  value = module.folder_tenants.id
}

output "cluster_prod" {
  value = module.gke_cluster_hmt_prod
}
