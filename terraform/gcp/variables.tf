variable "bigquery-project-id" {
  type        = string
  description = "The project ID to retrieve bigquery metrics from"
  default     = ""
}

variable "datadog-api-key-secret" {
  type        = string
  description = "Name of the secret containing the Datadog API key stored in Google Secret Manager"
}

variable "image-repository" {
  type        = string
  description = "The repository where the image is stored"
  default     = "ovotech/bigquery-metrics-extractor"
}

variable "image-tag" {
  type        = string
  description = "The version of the image to launch"
  default     = "latest"
}

variable "machine-type" {
  type        = string
  description = "The type of the instance to run bqmetrics service on"
  default     = "e2-small"
}

variable "metric-interval" {
  type        = string
  description = "The interval between metric submission"
  default     = "30s"
}

variable "metric-tags" {
  type        = map(string)
  description = "The tags to attach on metrics"
  default     = {}
}

variable "network-tags" {
  type        = list(string)
  description = "Network tags to apply on the bqmetrics instance"
  default     = []
}

variable "project" {
  type        = string
  description = "The project in which to run the bqmetrics service"
  default     = ""
}

variable "region" {
  type        = string
  description = "The region to run the bqmetrics service in"
  default     = ""
}

variable "zone" {
  type        = string
  description = "The zone to run the bqmetrics service in"
  default     = ""
}

variable "service-account-email" {
  type        = string
  description = "The service account email to run the bqmetrics service under"
  default     = ""
}

variable "subnetwork" {
  type        = string
  description = "The subnetwork to connect the bqmetrics instance to"
}

data "google_client_config" "current" {}
data "google_compute_zones" "available" {
  region = local.region
}
data "google_secret_manager_secret_version" "datadog-api-key" {
  project = local.project
  secret  = var.datadog-api-key-secret
}

resource "random_shuffle" "zones" {
  input = data.google_compute_zones.available.names
}

locals {
  bigquery-project       = coalesce(var.bigquery-project-id, local.project)
  create-service-account = var.service-account-email == ""
  metric-tags            = join(",", sort([for k, v in var.metric-tags : (v == "" ? k : "${k}:${v}")]))
  project                = coalesce(var.project, data.google_client_config.current.project)
  region                 = coalesce(var.region, data.google_client_config.current.region)
  service-account-email  = local.create-service-account ? google_service_account.bqmetricsd.0.email : var.service-account-email
  zone                   = coalesce(var.zone, random_shuffle.zones.result[0])
}
