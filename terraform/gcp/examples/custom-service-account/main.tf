provider "google" {}

terraform {
  required_version = "0.12.26"

  required_providers {
    google = ">= 2.20.3"
    random = "~> 2.2"
  }
}

data "google_compute_subnetwork" "default" {
  name   = "default"
  region = "europe-west1"
}

module "bigquery-metrics-exporter" {
  source = "../.."

  datadog-api-key-secret = "datadog-api-key"
  subnetwork             = data.google_compute_subnetwork.default.self_link
  service-account-email  = google_service_account.custom.email
}

resource "google_service_account" "custom" {
  account_id = "bigquery-metrics"
}

# Give the service account permissions to get BigQuery table-level metrics
resource "google_project_iam_member" "bigquery-access" {
  member = "serviceAccount:${google_service_account.custom.email}"
  role   = "roles/bigquery.metadataViewer"
}

# Give the service account permissions to read the Datadog API key
resource "google_secret_manager_secret_iam_member" "secret-access" {
  member    = "serviceAccount:${google_service_account.custom.email}"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = "datadog-api-key"
}

# Give the service account permissions to write logs to Cloud Logging
resource "google_project_iam_member" "logs-write-access" {
  member = "serviceAccount:${google_service_account.custom.email}"
  role   = "roles/logging.logWriter"
}

# Give the service account permissions to write instance metrics to Monitoring
resource "google_project_iam_member" "metrics-write-access" {
  member = "serviceAccount:${google_service_account.custom.email}"
  role   = "roles/monitoring.metricWriter"
}
