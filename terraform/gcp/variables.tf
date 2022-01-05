variable "bigquery-project-id" {
  type        = string
  description = "The project ID to retrieve bigquery metrics from"
  default     = ""
}

variable "block-project-ssh-keys" {
  type        = bool
  description = "Block project-wide SSH keys from being able to connect to the instance"
  default     = true
}

variable "custom-metrics" {
  type        = any
  description = <<-EOT
  List of custom metric stanzas. Type is given as any as there are a number of optional components.
  Expected type is below:
  type = list(object({
    metric-name     = string
    metric-interval = optional(string)
    metric-tags     = optional(list(string))
    sql             = string
  }))
  EOT
  default     = []
}

variable "datadog-api-key-secret" {
  type        = string
  description = "Name of the secret containing the Datadog API key stored in Google Secret Manager"
}

variable "dataset-filter" {
  type        = string
  description = "A label to filter BigQuery datasets by when querying for table metrics"
  default     = ""
}

variable "enable-autohealing" {
  type        = bool
  description = "Enables autohealing using the healthcheck endpoint"
  default     = true
}

variable "enable-os-login" {
  type        = bool
  description = "Enables OS login on the instance"
  default     = false
}

variable "image-repository" {
  type        = string
  description = "The repository where the image is stored"
  default     = "ovotech/bigquery-metrics-exporter"
}

variable "image-tag" {
  type        = string
  description = "The version of the image to launch"
  default     = "latest"
}

variable "log-level" {
  type        = string
  description = "The log level of bqmetricsd. Should be one of debug, info, warn, error"
  default     = "info"
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

variable "metric-prefix" {
  type        = string
  description = "Optionally the prefix to give to metrics"
  default     = ""
}

variable "metric-tags" {
  type        = list(string)
  description = "The tags to attach on metrics"
  default     = []
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

variable "service-account-email" {
  type        = string
  description = "The service account email to run the bqmetrics service under"
  default     = ""
}

variable "stackdriver-logging" {
  type        = bool
  description = "Enables exporting of instance logs to Stackdriver (For bqmetrics service logs, etc.)"
  default     = false
}

variable "stackdriver-monitoring" {
  type        = bool
  description = "Enables exporting of instance metrics to Stackdriver (For monitoring of RAM usage of bqmetrics service, etc.)"
  default     = false
}

variable "subnetwork" {
  type        = string
  description = "The subnetwork to connect the bqmetrics instance to"
}

variable "zone" {
  type        = string
  description = "The zone to run the bqmetrics service in"
  default     = ""
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
  allow-bigquery-jobs    = length(var.custom-metrics) > 0
  bigquery-project       = coalesce(var.bigquery-project-id, local.project)
  config_path            = "/etc/bqmetrics/config.json"
  create-service-account = var.service-account-email == ""
  project                = coalesce(var.project, data.google_client_config.current.project)
  region                 = coalesce(var.region, data.google_client_config.current.region)
  service-account-email  = local.create-service-account ? google_service_account.bqmetricsd.0.email : var.service-account-email
  zone                   = coalesce(var.zone, random_shuffle.zones.result[0])
}
