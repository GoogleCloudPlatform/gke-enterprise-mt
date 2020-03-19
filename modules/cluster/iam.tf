# Assign compute network user role to kubernetes service account and google api service account
module "gke-subnet-iam-bindings" {
  source = "terraform-google-modules/iam/google//modules/subnets_iam"
  version        = "~> 5.0.0"
  subnets        = [ var.subnet_name ]
  subnets_region = var.region
  project        = var.host_project_id
  mode           = "authoritative"
  bindings = {
    "roles/compute.networkUser" = [
      "serviceAccount:service-${var.service_project_number}@container-engine-robot.iam.gserviceaccount.com",
      "serviceAccount:${var.service_project_number}@cloudservices.gserviceaccount.com",
    ]
  }
}

# Resources for service account bindings, assigning container.hostServiceAgentUser role
# to kubernetes service account and google api service account.
#
# terraform-google-modules/iam/google//modules/projects_iam is not used because it relies
# on for_each which cannot handle the project numbers in the service accounts.
# The google_project_iam_member resource is equivalent to an additive binding
# (google_project_iam_binding corresponds to authoritative).

resource "google_project_iam_member" "gke-sa-iam-bindings-container" {
  project = var.host_project_id
  role = "roles/container.hostServiceAgentUser"
  member = "serviceAccount:service-${var.service_project_number}@container-engine-robot.iam.gserviceaccount.com"
}

resource "google_project_iam_member" "gke-sa-iam-bindings-cloudservices" {
  project = var.host_project_id
  role = "roles/container.hostServiceAgentUser"
  member = "serviceAccount:${var.service_project_number}@cloudservices.gserviceaccount.com"
}
