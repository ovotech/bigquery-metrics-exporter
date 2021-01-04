package config

import (
	"context"
	"flag"
	"fmt"
	"golang.org/x/oauth2/google"
	"io/ioutil"
	"os"
	"strings"
	"time"

	sm "cloud.google.com/go/secretmanager/apiv1"
	smpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// DefaultMetricPrefix is the prefix that metric names have by default
var DefaultMetricPrefix = "custom.gcp.bigquery.table"

// DefaultMetricInterval is the default period between table-level metric exports
var DefaultMetricInterval = "30s"

// Version is the version of the program
var Version = "0.0.0"

// Config contains application configuration details
type Config struct {
	DatadogAPIKey  string
	GcpProject     string
	MetricPrefix   string
	MetricTags     []string
	MetricInterval time.Duration
	Profiling      bool
}

// NewConfig returns a Config by merging in the values from environment variables
// and those presented via command line flags. It will return an error if any of
// the variables are of an incorrect format or if the created Config is not valid
func NewConfig(name string) (*Config, error) {
	cnf := &Config{}

	cl := argsFromCommandLine(name)
	envs := argsFromEnv()

	var err error

	cnf.DatadogAPIKey = envs.datadogAPIKey
	if cnf.DatadogAPIKey == "" {
		ddAPIKeyFile := coalesce(cl.datadogAPIKeyFile, envs.datadogAPIKeyFile)
		if ddAPIKeyFile != "" {
			cnf.DatadogAPIKey, err = getValueFromFile(ddAPIKeyFile)
			if err != nil {
				return nil, fmt.Errorf("error reading datadog API key file: %w", err)
			}
		}
	}
	if cnf.DatadogAPIKey == "" {
		ddAPIKeySecretID := coalesce(cl.datadogAPIKeySecretID, envs.datadogAPIKeySecretID)
		if ddAPIKeySecretID != "" {
			cnf.DatadogAPIKey, err = getValueFromSecretManager(ddAPIKeySecretID)
			if err != nil {
				return nil, fmt.Errorf("error reading datadog secret: %w", err)
			}
		}
	}

	cnf.GcpProject = coalesce(cl.projectID, envs.projectID)
	if cnf.GcpProject == "" {
		cnf.GcpProject, err = getDefaultProjectID()
		if err != nil {
			return nil, fmt.Errorf("error retrieving default Google project ID: %w", err)
		}
	}

	cnf.MetricPrefix = coalesce(cl.metricPrefix, envs.metricPrefix, DefaultMetricPrefix)
	cnf.MetricTags = parseTagString(coalesce(cl.metricTags, envs.metricTags))

	cnf.MetricInterval, err = time.ParseDuration(coalesce(cl.metricInterval, envs.metricInterval, DefaultMetricInterval))
	if err != nil {
		return nil, fmt.Errorf("error parsing metric interval: %w", err)
	}

	cnf.Profiling = cl.profiling

	err = ValidateConfig(cnf)
	if err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}

	return cnf, nil
}

// GetEnv will return the value of the environment variable if set, otherwise the default
func GetEnv(env, def string) string {
	if val, ok := os.LookupEnv(env); ok {
		return val
	}
	return def
}

// ValidateConfig will validate that all of the required config parameters are present
func ValidateConfig(c *Config) error {
	if c.DatadogAPIKey == "" {
		return ErrMissingDatadogAPIKey
	}

	if c.GcpProject == "" {
		return ErrMissingGcpProject
	}

	if c.MetricPrefix == "" {
		return ErrMissingMetricPrefix
	}

	if c.MetricInterval == time.Duration(0) {
		return ErrMissingMetricInterval
	}

	return nil
}

func getValueFromFile(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func getValueFromSecretManager(id string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := sm.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("error creating Google Secret Manager client: %w", err)
	}

	req := &smpb.AccessSecretVersionRequest{Name: id}
	resp, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("error accessing secret version: %w", err)
	}

	return string(resp.Payload.GetData()), nil
}

func getDefaultProjectID() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	auth, err := google.FindDefaultCredentials(ctx)
	if err != nil {
		return "", err
	}

	return auth.ProjectID, nil
}

type arguments struct {
	datadogAPIKey, datadogAPIKeyFile, datadogAPIKeySecretID, projectID, metricPrefix, metricInterval, metricTags string
	profiling bool
}

func argsFromEnv() arguments {
	args := arguments{}
	do := func(tgt *string, env string) {
		if val, ok := os.LookupEnv(env); ok {
			*tgt = val
		}
	}

	do(&args.datadogAPIKey, "DATADOG_API_KEY")
	do(&args.datadogAPIKeyFile, "DATADOG_API_KEY_FILE")
	do(&args.datadogAPIKeySecretID, "DATADOG_API_KEY_SECRET_ID")
	do(&args.projectID, "GCP_PROJECT_ID")
	do(&args.metricPrefix, "METRIC_PREFIX")
	do(&args.metricTags, "METRIC_TAGS")
	do(&args.metricInterval, "METRIC_INTERVAL")

	return args
}

func argsFromCommandLine(name string) arguments {
	args := arguments{}
	flags := flag.NewFlagSet(name, flag.ExitOnError)
	flags.StringVar(&args.datadogAPIKeyFile, "datadog-api-key-file", "", "File containing the Datadog API key")
	flags.StringVar(&args.datadogAPIKeySecretID, "datadog-api-key-secret-id", "", "Google Secret Manager Resource ID containing the Datadog API key")
	flags.StringVar(&args.projectID, "gcp-project-id", "", "The GCP project to extract BigQuery metrics from")
	flags.StringVar(&args.metricPrefix, "metric-prefix", "", fmt.Sprintf("The prefix for the metrics names exported to Datadog (Default %s)", DefaultMetricPrefix))
	flags.StringVar(&args.metricInterval, "metric-interval", "", fmt.Sprintf("The interval between metrics submissions (Default %s)", DefaultMetricInterval))
	flags.StringVar(&args.metricTags, "metric-tags", "", "Comma-delimited list of tags to attach to metrics")
	flags.BoolVar(&args.profiling, "enable-profiler", false, "Enables the profiler")

	_ = flags.Parse(os.Args[1:])

	return args
}

func coalesce(s ...string) string {
	for _, val := range s {
		if val != "" {
			return val
		}
	}
	return ""
}

func parseTagString(t string) []string {
	var tags []string

	for _, tag := range strings.FieldsFunc(t, func(x rune) bool {
		return x == ',' || x == ' '
	}) {
		tags = append(tags, tag)
	}

	return tags
}
