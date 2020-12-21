resource "google_service_account" "bqmetricsd" {
  count = local.create-service-account ? 1 : 0

  account_id   = "bqmetricsd-${random_string.suffix.result}"
  description  = "Service account that runs the bqmetricsd service"
  display_name = "bqmetricsd"
  project      = local.project
}

resource "google_project_iam_member" "bq-role" {
  count = local.create-service-account ? 1 : 0

  member  = "serviceAccount:${google_service_account.bqmetricsd.0.email}"
  role    = "roles/bigquery.metadataViewer"
  project = local.bigquery-project
}

resource "google_project_iam_member" "logger-role" {
  count = local.create-service-account ? 1 : 0

  member  = "serviceAccount:${google_service_account.bqmetricsd.0.email}"
  role    = "roles/logging.logWriter"
  project = local.project
}

resource "google_project_iam_member" "metric-role" {
  count = local.create-service-account ? 1 : 0

  member  = "serviceAccount:${google_service_account.bqmetricsd.0.email}"
  role    = "roles/monitoring.metricWriter"
  project = local.project
}

resource "google_secret_manager_secret_iam_member" "secret-role" {
  count = local.create-service-account ? 1 : 0

  member    = "serviceAccount:${google_service_account.bqmetricsd.0.email}"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = var.datadog-api-key-secret
  project   = local.project
}
