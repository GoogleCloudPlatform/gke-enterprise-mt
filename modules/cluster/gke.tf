module "gke_cluster" {
  source                           = "terraform-google-modules/kubernetes-engine/google//modules/beta-private-cluster"
  version                          = "~> 7.3.0"
  project_id                       = var.service_project_id
  name                             = var.cluster_name
  region                           = var.region
  zones                            = var.zones
  network                          = var.network_name
  network_project_id               = var.host_project_id
  subnetwork                       = var.subnet_name
  ip_range_pods                    = var.ip_range_pods
  ip_range_services                = var.ip_range_services
  http_load_balancing              = true
  horizontal_pod_autoscaling       = true
  network_policy                   = true
  enable_private_endpoint          = var.use_private_endpoints
  deploy_using_private_endpoint    = var.use_private_endpoints
  enable_private_nodes             = true
  master_ipv4_cidr_block           = var.master_ipv4_cidr_block
  identity_namespace               = "${var.service_project_id}.svc.id.goog"
  service_account                  = "create"
  resource_usage_export_dataset_id = module.gke_usage_meter_dataset.dataset_name
  authenticator_security_group     = var.gke_security_group
  enable_vertical_pod_autoscaling  = true
  remove_default_node_pool         = true
  cluster_ipv4_cidr                = null  # To avoid conflict with ip_allocation_policy
  enable_intranode_visibility      = var.enable_intranode_visibility

  pod_security_policy_config = [{ "enabled" = true }]

  master_authorized_networks = var.use_private_endpoints ? [
    {
      cidr_block   = var.master_authorized_networks
      display_name = "bastion-host"
    },
  ] : []

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

resource "google_compute_firewall" "bastion-ssh-firewall" {
  count = var.use_private_endpoints ? 1 :0

  name          = format("%s-bastion-ssh", var.cluster_name)
  network       = var.network_name
  direction     = "INGRESS"
  project       = var.host_project_id
  source_ranges = ["0.0.0.0/0"]

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  target_tags = ["bastion"]
}

// The user-data script on Bastion instance provisioning
// Note that the default tinyproxy config only allows connections from localhost 
// which means that ssh forwarded connections are not allowed.
data "template_file" "startup_script" {
  template = <<-EOF
  sudo apt-get update -y
  sudo apt-get install -y tinyproxy
  sudo sed -i 's/^Allow/#Allow/' /etc/tinyproxy/tinyproxy.conf
  sudo systemctl kill tinyproxy
  sudo systemctl start tinyproxy
  EOF

}

// The Bastion Host with tiny proxy installed for port forwarding
resource "google_compute_instance" "bastion" {
  count = var.use_private_endpoints ? 1 : 0

  name = format("%s-bastion-host", var.cluster_name)
  machine_type = "g1-small"
  zone = var.zones[0]
  project = var.host_project_id
  tags = ["bastion"]

  // Specify the Operating System Family and version.
  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-9"
    }
  }

  // Ensure that when the bastion host is booted, it will have tinyproxy
  metadata_startup_script = data.template_file.startup_script.rendered

  // Define a network interface in the correct subnet.
  network_interface {
    subnetwork = var.subnet_name

    // Add an ephemeral external IP.
    access_config {
      // Ephemeral IP
    }
  }

  // Allow the instance to be stopped by terraform when updating configuration
  allow_stopping_for_update = true
}
