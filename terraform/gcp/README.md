# Terraform GCP module
This Terraform module creates a GCE instance running the `bqmetricsd` service.

## Usage
```hcl
data "google_compute_subnetwork" "default" {
  name   = "default"
  region = "europe-west1"
}

module "bqmetrics" {
  source = "git::https://github.com/ovotech/bigquery-metrics-exporter.git//terraform/gcp?ref=v1.2.2"

  datadog-api-key-secret = "datadog-api-key"
  subnetwork             = data.google_compute_subnetwork.default.self_link
}
```

## Variables
#### bigquery-project-id (string)
Optional

The project ID to retrieve bigquery metrics from. Defaults to the same project
the instance is created in

#### block-project-ssh-keys (boolean)
Optional

Whether to block project-wide SSH keys from being able to connect to the 
`bqmetricsd` instance, as an enhanced security measure. Defaults to `true`.

#### custom-metrics (list(object))
Optional

List of custom metric stanzas to generate metrics from SQL queries run in
BigQuery. The expected type for these custom metrics is as below:
```hcl
type = list(object({
    metric-name     = string
    metric-interval = optional(string)
    metric-tags     = optional(list(string))
    sql             = string
}))
```
The SQL should return a single row of data, and each column within the returned
row is published as a metric tagged with the column name.

#### datadog-api-key-secret (string)
Required

Name of the secret containing the Datadog API key stored in Google Secret
Manager

#### dataset-filter (string)
Optional

Label to filter BigQuery datasets by when querying for table metrics. Should be
in the format `tag:value`. https://cloud.google.com/bigquery/docs/labels-intro

#### enable-os-login (boolean)
Optional

Whether to enable the OS Login feature on the instance or not. OS Login allows
users to connect to the instance so should ideally be used for debugging only.
Defaults to `false`. https://cloud.google.com/compute/docs/oslogin

#### enable-autohealing (boolean)
Optional

Whether to enable the autohealing feature of the Managed Instance Group or not.
Enabling autohealing will enable the healthcheck endpoint of `bqmetricsd` on
port 8080. Defaults to `true`. 

#### image-repository (string)
Optional

The repository where the image is stored. Defaults to 
"ovotech/bigquery-metrics-exporter"

#### image-tag (string)
Optional

The version of the image to launch. Defaults to "latest"

#### log-level (string)
Optional

The log level to set on `bqmetricsd`. Should be one of `debug`, `info`, `warn`,
`error`. Defaults to `info`.

#### machine-type (string)
Optional

The type of the instance to run bqmetrics service on. Defaults to "e2-small"

#### metric-interval (string)
Optional

The interval between metric submission. Defaults to "30s"

#### metric-prefix (string)
Optional

The prefix to give to metrics. If unset, will use the application 
default of to "custom.gcp.bigquery".

#### metric-tags (map(string))
Optional

The tags to attach on metrics. Defaults to an empty map

#### network-tags (list(string))
Optional

Network tags to apply on the bqmetrics instance. Defaults to an empty list

#### project (string)
Optional

The project in which to run the bqmetrics instance. Defaults to the project set in the 
provider

#### region (string)
Optional

The region to run the bqmetrics instance in. Defaults to the region set in the provider

#### service-account-email (string)
Optional

The service account email to run the bqmetrics service under. If not provided, a service
account with minimally required permissions will be automatically created.

#### stackdriver-logging (boolean)
Optional

Whether to enable exporting of instance logs to Stackdriver. Defaults to 
`false`.

#### stackdriver-monitoring (boolean)
Optional

Whether to enable the exporting of instance metrics to Stackdriver. Defaults to
`false`.

#### subnetwork (string)
Required

The subnetwork to connect the bqmetrics instance to

#### zone (string)
Optional

The zone to run the bqmetrics instance in. Defaults to a random zone
