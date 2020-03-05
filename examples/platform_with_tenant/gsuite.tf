resource "gsuite_group" "auditor" {
  email       = "auditor@${var.domain}"
  name        = "auditor"
  description = "auditor"
}

resource "gsuite_group" "cluster-admin" {
  email       = "cluster-admin@${var.domain}"
  name        = "cluster-admin"
  description = "cluster-admin"
}

resource "gsuite_group" "folder-admin" {
  email       = "folder-admin@${var.domain}"
  name        = "folder-admin"
  description = "folder-admin"
}

resource "gsuite_group" "network-admin" {
  email       = "network-admin@${var.domain}"
  name        = "network-admin"
  description = "network-admin"
}

resource "gsuite_group" "org-admin" {
  email       = "org-admin@${var.domain}"
  name        = "org-admin"
  description = "org-admin"
}

resource "gsuite_group" "security-admin" {
  email       = "security-admin@${var.domain}"
  name        = "security-admin"
  description = "security-admin"
}

resource "gsuite_group" "gke-security" {
  email       = "gke-security-groups@${var.domain}"
  name        = "gke-security-groups"
  description = "gke-security"
}

resource "gsuite_group" "tenant-admin" {
  email       = "tenant-admin@${var.domain}"
  name        = "tenant-admin"
  description = "tenant-admin"
}
