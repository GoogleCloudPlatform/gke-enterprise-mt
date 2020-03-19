// Random Id generator for unique dataset ID
resource "random_id" "random_bq_id_suffix" {
  byte_length = 2
}

locals {
  cleaned_prefix = replace(var.prefix, "-", "_")
}

module "gke_usage_meter_dataset" {
  source  = "terraform-google-modules/bigquery/google"
  version = "~> 2.0"
  dataset_id = format( "%s_gke_usage_meter_%s", local.cleaned_prefix, random_id.random_bq_id_suffix.hex)
  dataset_name = format( "%s_gke_usage_meter_%s", local.cleaned_prefix, random_id.random_bq_id_suffix.hex)
  description       = "GKE Usage meter dataset"
  project_id        = var.cluster_service_project_id
  location          = "US"
  time_partitioning = "DAY"
  dataset_labels = {
    env      = "prod"
  }
}
