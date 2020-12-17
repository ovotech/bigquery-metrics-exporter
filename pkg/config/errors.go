package config

import "errors"

var (
	// ErrMissingDatadogAPIKey is the error returned when the Config is missing an API key for Datadog
	ErrMissingDatadogAPIKey = errors.New("no Datadog API key configured")

	// ErrMissingGcpProject is the error returned when the Config is missing the Google project ID
	ErrMissingGcpProject = errors.New("no GCP project ID configured")

	// ErrMissingMetricPrefix is the error returned when the Config is missing a metric prefix
	ErrMissingMetricPrefix = errors.New("no metric name prefix configured")

	// ErrMissingMetricInterval is the error returned when the Config is missing a metric collection interval
	ErrMissingMetricInterval = errors.New("no metric collection interval configured")
)
