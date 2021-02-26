# bigquery-metrics-exporter

A Golang application to export table level metrics from BigQuery into Datadog.

Two binaries are provided. `bqmetrics` runs a single round of metrics
collection. `bqmetricsd` runs metrics collection continually according to the
provided metric collection interval.

## Metrics
The metrics exporter queries the BigQuery metadata API to generate metrics. The
metadata API only stores this information for **tables** and **materialized
views**, so views and external data sources will not have metrics exported.

The following metrics are generated:
* **row_count** - The number of rows in the table
* **last_modified** - The number of seconds since this table was last modified
* **last_modified_time** - The timestamp when the table was last modified

Inserting or modifying data in the table also updates the last modified time,
so those metrics can be used as a measure of data freshness.

## Custom Metrics
The metrics exporter also includes the ability to generate Datadog metrics from
the results of SQL queries.

:warning: *Running an SQL query on BigQuery may have a cost associated with it*

Each custom metric has a name, a list of tags, and its own collection interval
as well as the SQL query to run to produce the metrics. The SQL query should
return a single row of data, and each column will be exported as a distinct
metric.

## Recommended usage
It is recommended to run the metrics collection daemon `bqmetricsd` which will
continually collect metrics and ship them to Datadog according to the provided
schedule.
```
bqmetricsd \
  --datadog-api-key-file datadog.key \
  --gcp-project-id my-project \
  --metric-interval 1m \
  --metric-tags team:myteam,env:prod
```

### Running in Google Cloud Platform
Running in Google Cloud Platform is the preferred method of operation as it
will reduce latency for metrics collection and simplify authentication to the
BigQuery API. A [Terraform provider](terraform/gcp/README.md) is provided to
simplify running the daemon in GCP.

The Terraform provider makes use of Google Secrets Manager to handle the
Datadog API secret key. This secret can be created with the `gcloud` CLI
utility using the following command:
```shell
printf "secret" | gcloud secrets create datadog-api-key --data-file=-
```

Depending on organizational policy, you may need to restrict the secret to
certain locations. See `gcloud secrets create --help` for full details.

An existing secret can be updated with the following commands:
```shell
printf "secret" | gcloud secrets versions add datadog-api-key --data-file=-
```

## Configuration
`bqmetrics` and `bqmetricsd` are both configurable using the same mechanisms,
either a config file, environment variables, or on the command line. Config set
on the command line has priority over environment variables, which in turn have
priority over the config file.

It is required that the Datadog API key is set using one of the available 
options in order to run. Credentials also need to be provided for connecting 
to the GCP APIs, although that may be handled automatically by the environment.
See [the Google Cloud Platform authentication documentation](https://cloud.google.com/docs/authentication/production)
for more information. The Google Cloud Project ID is also required. All other
parameters are optional.

### Config file
See [example-config.yaml](./example-config.yaml) for an example config file
that details all the current configuration.

`bqmetrics` and `bqmetricsd` will by default search for a config file at
`/etc/bqmetrics/config.yaml` and `~/.bqmetrics/config.yaml`, although you can
also specify the path to a config file using the `--config-file` command line
parameter or the `CONFIG_FILE` environment variable.

### Environment and command line parameters
Below is a list of configuration available as environment variables and command
line options.

| Environment Variable | Parameter | Description |
| --- | --- | --- |
| CONFIG_FILE | --config-file | Path to the config file |
| DATADOG_API_KEY |  | The Datadog API key |
| DATADOG_API_KEY_FILE | --datadog-api-key-file | File containing Datadog API key |
| DATADOG_API_KEY_SECRET_ID | --datadog-api-key-secret-id | Path to a secret held in Google Secret Manager containing Datadog API key, e.g. `projects/my-project/secrets/datadog-api-key/versions/3` |
| GCP_PROJECT_ID | --gcp-project-id | (Required) The Google Cloud project containing the BigQuery tables to retrieve metrics from |
| GOOGLE_APPLICATION_CREDENTIALS | | File containing service account details to authenticate to Google Cloud using |
| LOG_LEVEL | | The logging level (e.g. trace, debug, info, warn, error). Defaults to *info* |
| METRIC_INTERVAL | --metric-interval | The interval between metric collection rounds. Must contain a unit and valid units are "ns", "us" (or "µs"), "ms", "s", "m", "h". Defaults to *30s* |
| METRIC_PREFIX | --metric-prefix | The prefix for the metric names exported to Datadog. Defaults to *custom.gcp.bigquery* |
| METRIC_TAGS | --metric-tags | Comma-delimited list of tags to attach to metrics (e.g. env:prod,team:myteam) |

### GCP Service Account permissions
The service account running `bqmetricsd` may require the following roles:
```
BigQuery Metadata Viewer
    Required to generate table level metrics
Secret Manager Secret Accessor
    Required to access the Datadog API key if stored in Secret Manager
```
