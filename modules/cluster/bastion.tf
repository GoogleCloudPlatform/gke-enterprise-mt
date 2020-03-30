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
    subnetwork_project = var.host_project_id

    // Add an ephemeral external IP.
    access_config {
      // Ephemeral IP
    }
  }

  // Allow the instance to be stopped by terraform when updating configuration
  allow_stopping_for_update = true
}
