# bigquery-metrics-exporter

A Golang application to export table level metrics from BigQuery into Datadog.

## Configuration
`bqmetrics` and `bqmetrics` take the same optional parameters.

It is required that the Datadog API key is set using one of the below options
in order to run. Credentials also need to be provided for connecting to the
GCP APIs, although that may be handled automatically by the environment. See
[the Google authentication documentation](https://cloud.google.com/docs/authentication/production) 
for more information. All other parameters are optional.

```
--datadog-api-key-file
    File containing the Datadog API key
--datadog-api-key-secret-id
    Google Secret Manager Resource ID containing the Datadog API key
--gcp-project-id
    The GCP project to extract BigQuery metrics from
--metric-interval
    The interval between metrics submissions (Default 30s)
--metric-prefix
    The prefix for the metric names exported to Datadog (Default custom.gcp.bigquery.table)
--metric-tags
    Comma-delimited list of tags to attach to metrics
```

All parameters can be supplied as environment variables instead, and there
are additional environment variables that are not available as parameters.
Configuration supplied as parameters to the command takes precedence over
environment variables.

```
DATADOG_API_KEY
    The Datadog API key
DATADOG_API_KEY_FILE
    File containing the Datadog API key
DATADOG_API_KEY_SECRET_ID
    Google Secret Manager Resource ID containing the Datadog API key
GCP_PROJECT_ID
    The GCP project to extract BigQuery metrics from
GOOGLE_APPLICATION_CREDENTIALS
    File containing the service account details to authenticate to GCP using
LOG_LEVEL
    The logging level (e.g. trace, debug, info, warn, error)
METRIC_INTERVAL
    The interval between metrics submissions (Default 30s)
METRIC_PREFIX
    The prefix for the metric names exported to Datadog (Default custom.gcp.bigquery.table)
METRIC_TAG
    Comma-delimited list of tags to attach to metrics
```

### GCP Service Account permissions
The service account may require the following roles:
```
BigQuery Metadata Viewer
    Required to generate table level metrics
Secret Manager Secret Accessor
    Required to access the Datadog API key if stored in Secret Manager
```
