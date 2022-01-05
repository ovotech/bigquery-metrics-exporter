provider "google" {}

terraform {
  required_version = ">= 0.12.26"

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
  metric-tags            = ["env:nonprod", "team:my_team"]
  custom-metrics = [
    {
      metric-name     = "cardinality"
      metric-tags     = ["project_id:my_project", "dataset_id:my_dataset", "table_id:my_table"]
      metric-interval = "5m"
      sql             = <<-EOT
        SELECT
          APPROX_COUNT_DISTINCT(`column_1`) `column_1`,
          APPROX_COUNT_DISTINCT(`column_2`) `column_2`,
          APPROX_COUNT_DISTINCT(`column_3`) `column_3`
        FROM
          `my_project.my_dataset.my_table`
      EOT
    },
    {
      metric-name = "random"
      sql         = <<-EOT
        SELECT RAND() `random`
      EOT
    }
  ]
}
