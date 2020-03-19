locals {
  cleaned_prefix = replace(var.prefix, "-", "_")
  cleaned_cluster_name = replace(var.cluster_name, "-", "_")
}

resource "random_id" "random_bq_id_suffix" {
  byte_length = 2
}

module "gke_usage_meter_dataset" {
  source  = "terraform-google-modules/bigquery/google"
  version = "~> 2.0"
  dataset_id = format("%s_%s_usage_%s", local.cleaned_prefix, local.cleaned_cluster_name, random_id.random_bq_id_suffix.hex)
  dataset_name = format("%s_%s_usage_%s", local.cleaned_prefix, local.cleaned_cluster_name, random_id.random_bq_id_suffix.hex)
  description       = "GKE Usage meter dataset"
  project_id        = var.service_project_id
  location          = "US"
  time_partitioning = "DAY"
  dataset_labels    = var.dataset_labels
}
