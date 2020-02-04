module "gke_cluster_hmt_prod" {
  source                     = "terraform-google-modules/kubernetes-engine/google//modules/beta-private-cluster"
  version                    = "~> 7.0.0"
  project_id                 = module.project_hmt_prod_cluster_service.project_id
  name                       = var.prod_cluster_name
  region                     = var.region
  zones                      = var.zones
  network                    = module.vpc_shared_net_prod.network_name
  network_project_id         = module.project_hmt_prod_cluster_host_prod.project_id
  subnetwork                 = module.vpc_shared_net_prod.subnets_names[0]
  ip_range_pods              = var.ip_range_pods
  ip_range_services          = var.ip_range_services
  http_load_balancing        = true
  horizontal_pod_autoscaling = true
  network_policy             = true
  enable_private_endpoint    = true
  enable_private_nodes       = true
  master_ipv4_cidr_block     = var.master_ipv4_cidr_block
  identity_namespace         = "${module.project_hmt_prod_cluster_service.project_id}.svc.id.goog"
  service_account            = "create"
  resource_usage_export_dataset_id = module.gke_usage_meter_dataset.dataset_name
  authenticator_security_group = "gke-security-groups@${var.domain}"
  enable_vertical_pod_autoscaling = true
  remove_default_node_pool   = true
  cluster_ipv4_cidr          = null  # To avoid conflict with ip_allocation_policy

  pod_security_policy_config = [{
    "enabled" = true
  }]

  master_authorized_networks = [
    {
      cidr_block   = module.vpc_shared_net_prod.subnets_ips[0]
      display_name = "bastion-host"
    },
  ]

  node_metadata = "GKE_METADATA_SERVER"
  node_pools = [
    {
      name               = "default-node-pool"
      machine_type       = "n1-standard-1"
      min_count          = 1
      max_count          = 2
      disk_size_gb       = 100
      disk_type          = "pd-standard"
      image_type         = "COS_CONTAINERD"
      auto_repair        = true
      auto_upgrade       = true
      preemptible        = false
      initial_node_count = 1
      sandbox_enabled    = true
    },
  ]

  logging_service    = "logging.googleapis.com/kubernetes"
  monitoring_service = "monitoring.googleapis.com/kubernetes"
  node_pools_oauth_scopes = {
    all = []

    default-node-pool = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
  }

  node_pools_labels = {
    all = {}

    default-node-pool = {
      default-node-pool = true
    }
  }

  node_pools_metadata = {
    all = {}

    default-node-pool = {
      node-pool-metadata-custom-value = "my-node-pool"
    }
  }

  node_pools_tags = {
    all = []

    default-node-pool = [
     "default-node-pool",
    ]
  }
}
