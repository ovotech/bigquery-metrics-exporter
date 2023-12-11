package config

import "errors"

var (
	// ErrMissingDatadogAPIKey is the error returned when the Config is missing an API key for Datadog
	ErrMissingDatadogAPIKey = errors.New("no Datadog API key configured")
	ErrInvalidDatadogSite   = errors.New("invalid Datadog site configured")

	// ErrMissingGcpProject is the error returned when the Config is missing the Google project ID
	ErrMissingGcpProject = errors.New("no GCP project ID configured")

	// ErrMissingMetricPrefix is the error returned when the Config is missing a metric prefix
	ErrMissingMetricPrefix = errors.New("no metric name prefix configured")

	// ErrMissingMetricInterval is the error returned when the Config is missing a metric collection interval
	ErrMissingMetricInterval = errors.New("no metric collection interval configured")

	// ErrMissingMetricName is the error returned when a CustomMetric is missing a metric name
	ErrMissingMetricName = errors.New("no metric name configured")

	// ErrMissingCustomMetricSQL is the error returned when a CustomMetric is missing SQL
	ErrMissingCustomMetricSQL = errors.New("no custom metric sql query configured")

	// ErrInvalidPort is the error returned when an invalid port is specified
	ErrInvalidPort = errors.New("invalid port specified")
)
