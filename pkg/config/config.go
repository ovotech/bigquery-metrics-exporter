package config

import (
	"context"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/google"
	"io/ioutil"
	"os"
	"strings"
	"time"

	sm "cloud.google.com/go/secretmanager/apiv1"
	smpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

const (
	configName = "config"
	tagName    = "viper"
)

// DefaultMetricPrefix is the prefix that metric names have by default
var DefaultMetricPrefix = "custom.gcp.bigquery.table"

// DefaultMetricInterval is the default period between table-level metric exports
var DefaultMetricInterval = "30s"

// Version is the version of the program
var Version = "0.0.0"

// Config holds the configuration for the application
type Config struct {
	DatadogAPIKey  string        `viper:"datadog-api-key"`
	GcpProject     string        `viper:"gcp-project-id"`
	MetricPrefix   string        `viper:"metric-prefix"`
	MetricTags     []string      `viper:"metric-tags"`
	MetricInterval time.Duration `viper:"metric-interval"`
	Profiling      bool          `viper:"enable-profiler"`
}

// NewConfig creates a config struct using the package viper for configuration
// construction. Configuration can either be passed in a config file, as flags
// when running the application, or as environment variables. Priority is as
// determined by the viper package.
func NewConfig(name string) (*Config, error) {
	var cfg Config
	var err error

	fs := configFlags(name)

	vpr := viper.New()

	vpr.AddConfigPath("/etc/bqmetrics")
	vpr.AddConfigPath("$HOME/.bqmetrics")
	vpr.SetConfigName(configName)

	handleEnvBindings(vpr, fs)

	cfgFile, _ := fs.GetString("config")
	if cfgFile != "" {
		vpr.SetConfigFile(cfgFile)
	}

	if err = vpr.BindPFlags(fs); err != nil {
		return nil, fmt.Errorf("failed to bind flags: %w", err)
	}

	if err = vpr.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok || cfgFile != "" {
			return nil, fmt.Errorf("failed to read in config: %w", err)
		}
	}

	if err = handleAliases(vpr, "datadog-api-key"); err != nil {
		return nil, err
	}

	if err = vpr.Unmarshal(&cfg, func(cfg *mapstructure.DecoderConfig) {
		cfg.TagName = tagName
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err = handleFinalDefaults(&cfg); err != nil {
		return nil, fmt.Errorf("could not handle defaults: %w", err)
	}

	if err = ValidateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}

	return &cfg, nil
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

// GetEnv will return the value of the environment variable if set, otherwise the default
func GetEnv(env, def string) string {
	if val, ok := os.LookupEnv(env); ok {
		return val
	}
	return def
}

func configFlags(name string) *pflag.FlagSet {
	defInterval, err := time.ParseDuration(DefaultMetricInterval)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to parse default metric interval")
	}

	flags := pflag.NewFlagSet(name, pflag.ExitOnError)
	flags.String("config", "", "Path to the config file")
	flags.String("datadog-api-key-file", "", "File containing the Datadog API key")
	flags.String("datadog-api-key-secret-id", "", "Google Secret Manager Resource ID containing the Datadog API key")
	flags.String("gcp-project-id", "", "The GCP project to extract BigQuery metrics from")
	flags.String("metric-prefix", DefaultMetricPrefix, fmt.Sprintf("The prefix for the metrics names exported to Datadog (Default %s)", DefaultMetricPrefix))
	flags.Duration("metric-interval", defInterval, fmt.Sprintf("The interval between metrics submissions (Default %s)", DefaultMetricInterval))
	flags.StringSlice("metric-tags", []string{}, "Comma-delimited list of tags to attach to metrics")
	flags.Bool("enable-profiler", false, "Enables the profiler")

	_ = flags.Parse(os.Args[1:])

	return flags
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

func handleAliases(vpr *viper.Viper, target string) error {
	if path := vpr.GetString(fmt.Sprintf("%s-file", target)); path != "" {
		if val, err := getValueFromFile(path); err != nil {
			return fmt.Errorf("failed to handle file alias: %w", err)
		} else {
			vpr.Set(target, val)
		}
	}

	if id := vpr.GetString(fmt.Sprintf("%s-secret-id", target)); id != "" {
		if val, err := getValueFromSecretManager(id); err != nil {
			return fmt.Errorf("failed to handle secret manager alias: %w", err)
		} else {
			vpr.Set(target, val)
		}
	}

	return nil
}

func handleEnvBindings(vpr *viper.Viper, fs *pflag.FlagSet) {
	// This parameter is not available as a flag so bind it separately
	_ = vpr.BindEnv("datadog-api-key", "DATADOG_API_KEY")

	fs.VisitAll(func(f *pflag.Flag) {
		env := strings.ReplaceAll(f.Name, "-", "_")
		_ = vpr.BindEnv(f.Name, strings.ToUpper(env))
	})
}

func handleFinalDefaults(cfg *Config) error {
	if cfg.GcpProject == "" {
		if def, err := getDefaultProjectID(); err != nil {
			return nil
		} else {
			cfg.GcpProject = def
		}
	}

	return nil
}
