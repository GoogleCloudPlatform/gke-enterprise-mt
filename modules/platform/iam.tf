# TODO(mattcary): the container-engine-robot accounts seem to be slow to create and
# only appear after a little while. So things can't get created consistently.

# module to assign compute network user role to kubernetes service account and google api service account
module "gke-subnet-iam-bindings" {
  source = "terraform-google-modules/iam/google//modules/subnets_iam"
  version        = "~> 5.0.0"
  subnets        = ["${module.vpc_shared_net_prod.subnets_names[0]}"]
  subnets_region = var.region
  project        = var.cluster_host_project_id
  mode           = "authoritative"
  bindings = {
    "roles/compute.networkUser" = [
      "serviceAccount:service-${var.cluster_service_project_number}@container-engine-robot.iam.gserviceaccount.com",
      "serviceAccount:${var.cluster_service_project_number}@cloudservices.gserviceaccount.com",
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
  project = var.cluster_host_project_id
  role = "roles/container.hostServiceAgentUser"
  member = "serviceAccount:service-${var.cluster_service_project_number}@container-engine-robot.iam.gserviceaccount.com"
}

resource "google_project_iam_member" "gke-sa-iam-bindings-cloudservices" {
  project = var.cluster_host_project_id
  role = "roles/container.hostServiceAgentUser"
  member = "serviceAccount:${var.cluster_service_project_number}@cloudservices.gserviceaccount.com"
}

resource "google_project_iam_member" "tenant-iam-bindings-admin" {
  project = var.cluster_service_project_id
  role = "projects/${var.cluster_service_project_id}/roles/${google_project_iam_custom_role.tenant-custom-role.role_id}"
  member = "group:${var.tenant_admin_group}"
}

resource "google_project_iam_custom_role" "tenant-custom-role" {
  project     = var.cluster_service_project_id
  role_id     = "hmtTenantRole"
  title       = "HMT Tenant Role"
  description = "HMT Tenant Role"
  permissions = ["container.apiServices.get", "container.apiServices.list", "container.clusters.get", "container.clusters.getCredentials"]
}

#Module to create service accounts for workload identity demo
module "workload_identity_demo_service_accounts" {
  source        = "terraform-google-modules/service-accounts/google"
  version       = "~> 2.0"
  project_id    = var.cluster_service_project_id
  names         = ["tenant-a-gsa", "tenant-b-gsa"]
  project_roles = [
    "${var.cluster_service_project_id}=>roles/iam.workloadIdentityUser",
  ]
}

#Module to assign IAM configurations at Org level to the org admin group
module "hmt_org_admin_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/resourcemanager.organizationAdmin" = [
      "group:${var.org_admin_group}",
    ]
    "roles/billing.creator" = [
      "group:${var.org_admin_group}",
    ]
    "roles/billing.user" = [
      "group:${var.org_admin_group}",
    ]
    "roles/compute.xpnAdmin" = [
      "group:${var.org_admin_group}",
    ]
    "roles/resourcemanager.projectCreator" = [
      "group:${var.org_admin_group}",
    ]
  }
}

#Module to assign IAM configurations at Org level to the folder admin group
module "hmt_folder_admin_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/resourcemanager.folderAdmin" = [
      "group:${var.folder_admin_group}",
    ]
    "roles/resourcemanager.projectCreator" = [
      "group:${var.folder_admin_group}",
    ]
    "roles/billing.user" = [
      "group:${var.folder_admin_group}",
    ]
  }
}

#Module to assign IAM configurations at Org level to the network admin group
module "hmt_network_admin_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/compute.networkAdmin" = [
      "group:${var.network_admin_group}",
    ]
  }
}

#Module to assign IAM configurations at Org level to the security admin group
module "hmt_security_admin_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/compute.securityAdmin" = [
      "group:${var.security_admin_group}",
    ]
  }
}

#Module to assign IAM configurations at Org level to auditor group
module "hmt_auditor_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/logging.privateLogViewer" = [
      "group:${var.auditor_group}",
    ]
  }
}

#Module to assign IAM configurations at Org level to the cluster admin group
module "hmt_cluster_admin_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/container.clusterAdmin" = [
      "group:${var.cluster_admin_group}",
    ]
  }
}
