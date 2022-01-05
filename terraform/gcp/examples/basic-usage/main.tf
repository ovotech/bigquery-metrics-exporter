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
}
