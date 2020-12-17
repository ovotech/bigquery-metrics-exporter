package config

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

var DefaultMetricPrefix = "custom.gcp.bigquery.table"
var DefaultMetricInterval = "30s"

type Config struct {
	DatadogApiKey  string
	GcpProject     string
	MetricPrefix   string
	MetricTags     []string
	MetricInterval time.Duration
}

// NewConfig returns a Config by merging in the values from environment variables
// and those presented via command line flags. It will return an error if any of
// the variables are of an incorrect format or if the created Config is not valid
func NewConfig(name string) (*Config, error) {
	cnf := &Config{}

	cl := argsFromCommandLine(name)
	envs := argsFromEnv()

	cnf.DatadogApiKey = envs.datadogApiKey
	if cnf.DatadogApiKey == "" {
		ddApiKeyFile := coalesce(cl.datadogApiKeyFile, envs.datadogApiKeyFile)
		if ddApiKeyFile != "" {
			content, err := ioutil.ReadFile(ddApiKeyFile)
			if err != nil {
				return nil, fmt.Errorf("error reading datadog API key file: %w", err)
			}

			cnf.DatadogApiKey = string(content)
		}
	}

	cnf.GcpProject = coalesce(cl.projectId, envs.projectId)
	cnf.MetricPrefix = coalesce(cl.metricPrefix, envs.metricPrefix, DefaultMetricPrefix)
	cnf.MetricTags = parseTagString(coalesce(cl.metricTags, envs.metricTags))

	var err error
	cnf.MetricInterval, err = time.ParseDuration(coalesce(cl.metricInterval, envs.metricInterval, DefaultMetricInterval))
	if err != nil {
		return nil, fmt.Errorf("error parsing metric interval: %w", err)
	}

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
	if c.DatadogApiKey == "" {
		return ErrMissingDatadogApiKey
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

type arguments struct {
	datadogApiKey, datadogApiKeyFile, projectId, metricPrefix, metricInterval, metricTags string
}

func argsFromEnv() arguments {
	args := arguments{}
	do := func(tgt *string, env string) {
		if val, ok := os.LookupEnv(env); ok {
			*tgt = val
		}
	}

	do(&args.datadogApiKey, "DATADOG_API_KEY")
	do(&args.datadogApiKeyFile, "DATADOG_API_KEY_FILE")
	do(&args.projectId, "GCP_PROJECT_ID")
	do(&args.metricPrefix, "METRIC_PREFIX")
	do(&args.metricTags, "METRIC_TAGS")
	do(&args.metricInterval, "METRIC_INTERVAL")

	return args
}

func argsFromCommandLine(name string) arguments {
	args := arguments{}
	flags := flag.NewFlagSet(name, flag.ExitOnError)
	flags.StringVar(&args.datadogApiKeyFile, "datadog-api-key-file", "", "File containing the Datadog API key")
	flags.StringVar(&args.projectId, "gcp-project-id", "", "The GCP project to extract BigQuery metrics from")
	flags.StringVar(&args.metricPrefix, "metric-prefix", "", fmt.Sprintf("The prefix for the metrics names exported to Datadog (Default %s)", DefaultMetricPrefix))
	flags.StringVar(&args.metricInterval, "metric-interval", "", fmt.Sprintf("The interval between metrics submissions (Default %s", DefaultMetricInterval))
	flags.StringVar(&args.metricTags, "metric-tags", "", "Comma-delimited list of tags to attach to metrics")

	_ = flags.Parse(os.Args[1:])

	return args
}

func coalesce(vals ...string) string {
	for _, val := range vals {
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
