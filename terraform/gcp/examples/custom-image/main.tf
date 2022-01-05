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
  subnetwork             = data.google_compute_subnetwork.default.id
  image-repository       = "${data.google_container_registry_repository.registry.repository_url}/bigquery-metrics-exporter"
}

resource "google_container_registry" "registry" {
  location = "eu"
}

data "google_container_registry_repository" "registry" {
  region = "eu"
}

resource "google_storage_bucket_iam_member" "bqmetricsd-allow-pull" {
  bucket = google_container_registry.registry.id
  member = "serviceAccount:${module.bigquery-metrics-exporter.service-account-email}"
  role   = "roles/storage.objectViewer"
}
