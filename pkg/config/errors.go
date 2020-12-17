package config

import "errors"

var (
	ErrMissingDatadogApiKey  = errors.New("no Datadog API key configured")
	ErrMissingGcpProject     = errors.New("no GCP project ID configured")
	ErrMissingMetricPrefix   = errors.New("no metric name prefix configured")
	ErrMissingMetricInterval = errors.New("no metric collection interval configured")
)
