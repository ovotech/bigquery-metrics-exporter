resource "random_string" "suffix" {
  length  = 4
  special = false
  upper   = false
}

module "container" {
  #checkov:skip=CKV_TF_1
  source  = "terraform-google-modules/container-vm/google"
  version = "~> 2.0"

  container = {
    image = "${var.image-repository}:${var.image-tag}"
    args  = ["--config-file=${local.config_path}"]
    env = [{
      name  = "LOG_LEVEL"
      value = var.log-level
    }]
    volumeMounts = [{
      mountPath = local.config_path
      name      = "config"
      readOnly  = true
    }]
  }

  volumes = [{
    name = "config"
    hostPath = {
      path = local.config_path
    }
  }]

  restart_policy = "Always"
}

resource "google_compute_instance_template" "bqmetricsd" {
  machine_type = var.machine-type
  name_prefix  = "bqmetricsd-"
  project      = local.project
  region       = local.region

  disk {
    auto_delete  = true
    boot         = true
    source_image = module.container.source_image
  }

  network_interface {
    subnetwork = var.subnetwork
  }

  labels = {
    (module.container.vm_container_label_key) = module.container.vm_container_label
  }

  # checkov:skip=CKV_GCP_32:Configurable but defaults to true
  metadata = merge(
    { (module.container.metadata_key) = module.container.metadata_value },
    var.block-project-ssh-keys ? { block-project-ssh-keys = "true" } : {},
    var.enable-os-login ? { enable-oslogin = "true" } : {},
    var.stackdriver-monitoring ? { google-monitoring-enabled = "true" } : {},
    var.stackdriver-logging ? { google-logging-enabled = "true" } : {},
  )

  metadata_startup_script = data.template_file.startup.rendered

  service_account {
    email  = local.service-account-email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }

  shielded_instance_config {
    enable_integrity_monitoring = true
    enable_secure_boot          = true
    enable_vtpm                 = true
  }

  tags = var.network-tags

  lifecycle {
    create_before_destroy = true
  }
}

resource "google_compute_instance_group_manager" "bqmetricsd" {
  base_instance_name = "bqmetricsd-${random_string.suffix.result}"
  description        = "Manages the deployment of the bqmetricsd service"
  name               = "bqmetricsd-grp-${random_string.suffix.result}"
  project            = local.project
  target_size        = 1
  wait_for_instances = true
  zone               = local.zone

  dynamic "auto_healing_policies" {
    for_each = var.enable-autohealing ? google_compute_health_check.autohealing[*].id : []

    content {
      health_check      = auto_healing_policies.value
      initial_delay_sec = 300
    }
  }

  update_policy {
    minimal_action  = "REPLACE"
    type            = "PROACTIVE"
    max_surge_fixed = 1
  }

  version {
    name              = "bqmetricsd"
    instance_template = google_compute_instance_template.bqmetricsd.id
  }
}

resource "google_compute_health_check" "autohealing" {
  count = var.enable-autohealing ? 1 : 0

  name                = "bqmetricsd-${random_string.suffix.result}-autohealing"
  check_interval_sec  = 5
  timeout_sec         = 5
  healthy_threshold   = 1
  unhealthy_threshold = 6

  http_health_check {
    request_path = "/health"
    port         = "8080"
  }
}

data "google_compute_subnetwork" "subnetwork" {
  name    = local.subnetwork-parts["name"]
  region  = local.subnetwork-parts["region"]
  project = local.subnetwork-parts["project"]
}

// Firewall rule to allow access to the bqmetrics healthcheck endpoint
// from the GCP source IPs https://cloud.google.com/load-balancing/docs/health-check-concepts#ip-ranges
resource "google_compute_firewall" "autohealing" {
  count = var.enable-autohealing ? 1 : 0

  name    = "bqmetricsd-${random_string.suffix.result}-autohealing"
  network = data.google_compute_subnetwork.subnetwork.network

  target_service_accounts = [local.service-account-email]
  source_ranges           = ["35.191.0.0/16", "130.211.0.0/22"]

  allow {
    protocol = "tcp"
    ports    = [8080]
  }
}

locals {
  config_init = {
    custom-metrics            = var.custom-metrics
    datadog-api-key-secret-id = data.google_secret_manager_secret_version.datadog-api-key.id
    dataset-filter            = var.dataset-filter
    gcp-project-id            = local.bigquery-project
    metric-interval           = var.metric-interval
    metric-prefix             = var.metric-prefix
    metric-tags               = var.metric-tags
    healthcheck = {
      enabled = var.enable-autohealing
      port    = 8080
    }
  }
  config = { for k, v in local.config_init : k => v if v != "" }
}

data "template_file" "startup" {
  template = file("${path.module}/templates/startup.sh")
  vars = {
    config_content = base64encode(jsonencode(local.config))
    config_path    = local.config_path
  }
}
