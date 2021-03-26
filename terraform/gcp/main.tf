resource "random_string" "suffix" {
  length  = 4
  special = false
  upper   = false
}

module "container" {
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

  metadata = merge(
    { (module.container.metadata_key) = module.container.metadata_value },
    var.stackdriver-monitoring ? { google-monitoring-enabled = "true" } : {},
    var.stackdriver-logging ? { google-logging-enabled = "true" } : {},
  )

  metadata_startup_script = data.template_file.startup.rendered

  service_account {
    email  = local.service-account-email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
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

locals {
  config_init = {
    custom-metrics            = var.custom-metrics
    datadog-api-key-secret-id = data.google_secret_manager_secret_version.datadog-api-key.id
    dataset-filter            = var.dataset-filter
    gcp-project-id            = local.bigquery-project
    metric-interval           = var.metric-interval
    metric-prefix             = var.metric-prefix
    metric-tags               = var.metric-tags
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
