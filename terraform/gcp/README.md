# Terraform GCP module
This Terraform module creates a GCE instance running the `bqmetricsd` service.

## Variables
#### bigquery-project (string)
Optional

The project ID to retrieve bigquery metrics from. Defaults to the same project
the instance is created in

#### datadog-api-key-secret (string)
Required

Name of the secret containing the Datadog API key stored in Google Secret
Manager

#### image-repository (string)
Optional

The repository where the image is stored. Defaults to 
"ovotech/bigquery-metrics-extractor"

#### image-tag (string)
Optional

The version of the image to launch. Defaults to "latest"

#### machine-type (string)
Optional

The type of the instance to run bqmetrics service on. Defaults to "e2-small"

#### metric-interval (string)
Optional

The interval between metric submission. Defaults to "30s"

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

#### zone (string)
Optional

The zone to run the bqmetrics instance in. Defaults to a random zone

#### service-account-email (string)
Optional

The service account email to run the bqmetrics service under. If not provided, a service
account with minimally required permissions will be automatically created.

#### subnetwork (string)
Required

The subnetwork to connect the bqmetrics instance to
