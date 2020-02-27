output "vpc_shared_net_prod" {
  value = module.vpc_shared_net_prod
}

output "cluster_prod" {
  value = module.gke_cluster
}
