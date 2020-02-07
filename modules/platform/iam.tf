# TODO(mattcary): the container-engine-robot accounts seem to be slow to create and
# only appear after a little while. So things can't get created consistently.

# module to assign compute network user role to kubernetes service account and google api service account
module "gke-subnet-iam-bindings" {
  source = "terraform-google-modules/iam/google//modules/subnets_iam"
  version        = "~> 5.0.0"
  subnets        = ["${module.vpc_shared_net_prod.subnets_names[0]}"]
  subnets_region = var.region
  project        = module.project_hmt_prod_cluster_host_prod.project_id
  mode           = "authoritative"
  bindings = {
    "roles/compute.networkUser" = [
      "serviceAccount:service-${module.project_hmt_prod_cluster_service.project_number}@container-engine-robot.iam.gserviceaccount.com",
      "serviceAccount:${module.project_hmt_prod_cluster_service.project_number}@cloudservices.gserviceaccount.com",
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
  project = module.project_hmt_prod_cluster_host_prod.project_id
  role = "roles/container.hostServiceAgentUser"
  member = "serviceAccount:service-${module.project_hmt_prod_cluster_service.project_number}@container-engine-robot.iam.gserviceaccount.com"
}

resource "google_project_iam_member" "gke-sa-iam-bindings-cloudservices" {
  project = module.project_hmt_prod_cluster_host_prod.project_id
  role = "roles/container.hostServiceAgentUser"
  member = "serviceAccount:${module.project_hmt_prod_cluster_service.project_number}@cloudservices.gserviceaccount.com"
}

resource "google_project_iam_member" "tenant-iam-bindings-admin" {
  project = module.project_hmt_prod_cluster_service.project_id
  role = "roles/${google_project_iam_custom_role.hmt-tenant-custom-role.role_id}"
  member = "group:hmt-tenant-admin@${var.domain}"
}

resource "google_project_iam_member" "tenant-iam-bindings-dev" {
  project = module.project_hmt_prod_cluster_service.project_id
  role = "roles/${google_project_iam_custom_role.hmt-tenant-custom-role.role_id}"
  member = "group:hmt-tenant-dev@${var.domain}"
}

#Module to assign project viewer role to RBAC groups
module "hmt_rbac_groups_project-iam-bindings" {
  source   = "terraform-google-modules/iam/google//modules/projects_iam"
  project = module.project_hmt_prod_cluster_service.project_id
  mode     = "additive"
  bindings = {
    "roles/viewer" = [
      "group:hmt-ns1-admin@${var.domain}",
      "group:hmt-ns1-dev@${var.domain}",
      "group:hmt-ns2-admin@${var.domain}",
      "group:hmt-ns2-dev@${var.domain}",
    ]
  }
}

resource "google_project_iam_custom_role" "hmt-tenant-custom-role" {
  project     = module.project_hmt_prod_cluster_service.project_id
  role_id     = "hmtTenantRole"
  title       = "HMT Tenant Role"
  description = "HMT Tenant Role"
  permissions = ["container.apiServices.get", "container.apiServices.list", "container.clusters.get", "container.clusters.getCredentials"]
}

#Module to create service accounts for workload identity demo
module "workload_identity_demo_service_accounts" {
  source        = "terraform-google-modules/service-accounts/google"
  version       = "~> 2.0"
  project_id    = module.project_hmt_prod_cluster_service.project_id
  names         = ["tenant-a-gsa", "tenant-b-gsa"]
  project_roles = [
    "${module.project_hmt_prod_cluster_service.project_id}=>roles/iam.workloadIdentityUser",
  ]
}

#Module to assign IAM configurations at Org level to hmt-org-admin group
module "hmt_org_admin_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/resourcemanager.organizationAdmin" = [
      "group:hmt-org-admin@${var.domain}",
    ]
    "roles/billing.creator" = [
      "group:hmt-org-admin@${var.domain}",
    ]
    "roles/billing.user" = [
      "group:hmt-org-admin@${var.domain}",
    ]
    "roles/compute.xpnAdmin" = [
      "group:hmt-org-admin@${var.domain}",
    ]
    "roles/resourcemanager.projectCreator" = [
      "group:hmt-org-admin@${var.domain}",
    ]
  }
}

#Module to assign IAM configurations at Org level to hmt-folder-admin group
module "hmt_folder_admin_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/resourcemanager.folderAdmin" = [
      "group:hmt-folder-admin@${var.domain}",
    ]
    "roles/resourcemanager.projectCreator" = [
      "group:hmt-folder-admin@${var.domain}",
    ]
    "roles/billing.user" = [
      "group:hmt-folder-admin@${var.domain}",
    ]
  }
}

#Module to assign IAM configurations at Org level to hmt-network-admin group
module "hmt_network_admin_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/compute.networkAdmin" = [
      "group:hmt-network-admin@${var.domain}",
    ]
  }
}

#Module to assign IAM configurations at Org level to hmt-security-admin group
module "hmt_security_admin_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/compute.securityAdmin" = [
      "group:hmt-security-admin@${var.domain}",
    ]
  }
}

#Module to assign IAM configurations at Org level to hmt-auditor group
module "hmt_auditor_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/logging.privateLogViewer" = [
      "group:hmt-auditor@${var.domain}",
    ]
  }
}

#Module to assign IAM configurations at Org level to hmt-cluster-admin group
module "hmt_cluster_admin_organization_iam_binding" {
  source        = "terraform-google-modules/iam/google//modules/organizations_iam"
  version       = "~> 5.0.0"
  organizations = [var.organization_id]
  mode          = "additive"

  bindings = {
    "roles/container.clusterAdmin" = [
      "group:hmt-cluster-admin@${var.domain}",
    ]
  }
}
