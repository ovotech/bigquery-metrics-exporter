package config

import (
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewConfig(t *testing.T) {
	setup := func(envs []string, args []string, keyfile string) func() {
		return func() {
			os.Clearenv()
			os.Args = append([]string{"./bqmetricstest"}, args...)
			for _, env := range envs {
				kvs := strings.SplitN(env, "=", 2)
				_ = os.Setenv(kvs[0], kvs[1])
			}
			if keyfile == "" {
				return
			}
			err := ioutil.WriteFile("/tmp/dd.key", []byte(keyfile), 0755)
			if err != nil {
				t.Fatalf("error when writing temporary key file: %v", err)
			}
		}
	}
	type args struct {
		t string
	}
	tests := []struct {
		name    string
		setup   func()
		args    args
		want    *Config
		wantErr bool
	}{
		{"all via env", setup([]string{"DATADOG_API_KEY=abc123", "GCP_PROJECT_ID=my-project-id", "METRIC_PREFIX=custom.gcp.bigquery.stats", "METRIC_TAGS=env:prod", "METRIC_INTERVAL=2m"}, nil, ""), args{"bqmetricstest"}, &Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: 2 * time.Minute,
			Profiling:      false,
		}, false},
		{"all via cmd", setup(nil, []string{"--datadog-api-key-file=/tmp/dd.key", "--gcp-project-id=my-project-id", "--metric-prefix=custom.gcp.bigquery.stats", "--metric-tags=env:prod", "--metric-interval=2m", "--enable-profiler"}, "abc123"), args{"bqmetricstest"}, &Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: 2 * time.Minute,
			Profiling:      true,
		}, false},
		{"mixture of sources", setup([]string{"DATADOG_API_KEY=abc123", "GCP_PROJECT_ID=my-project-id"}, []string{"--metric-prefix=custom.gcp.bigquery.stats", "--metric-tags=env:prod", "--metric-interval=2m"}, ""), args{"bqmetricstest"}, &Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: 2 * time.Minute,
			Profiling:      false,
		}, false},
		{"minimum required config", setup([]string{"DATADOG_API_KEY=abc123", "GCP_PROJECT_ID=my-project-id"}, nil, ""), args{"bqmetricstest"}, &Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   DefaultMetricPrefix,
			MetricTags:     nil,
			MetricInterval: 30 * time.Second,
			Profiling:      false,
		}, false},
		{"unreadable key file", setup([]string{"DATADOG_API_KEY_FILE=/tmp/not-found.key", "GCP_PROJECT_ID=my-project-id"}, nil, "abc123"), args{"bqmetricstest"}, nil, true},
		{"missing key", setup([]string{"GCP_PROJECT_ID=my-project-id"}, nil, ""), args{"bqmetricstest"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			got, err := NewConfig(tt.args.t)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	type args struct {
		c *Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"valid", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(30000),
		}}, false},
		{"missing datadog api key", args{&Config{
			DatadogAPIKey:  "",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(30000),
		}}, true},
		{"missing gcp project id", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(30000),
		}}, true},
		{"missing metric prefix", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(30000),
		}}, true},
		{"missing metric tags", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     nil,
			MetricInterval: time.Duration(30000),
		}}, false},
		{"missing metric interval", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(0),
		}}, true},
		{"missing profiling", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(30000),
			Profiling:      false,
		}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateConfig(tt.args.c); (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	setup := func(env, val string) func() {
		return func() {
			os.Clearenv()
			if env == "" {
				return
			}
			err := os.Setenv(env, val)
			if err != nil {
				t.Fatalf("could not set test environment: %v", err)
			}
		}
	}
	type args struct {
		env string
		def string
	}
	tests := []struct {
		name  string
		setup func()
		args  args
		want  string
	}{
		{"return env value", setup("TEST_ENV", "abc123"), args{"TEST_ENV", "default"}, "abc123"},
		{"return blank env value", setup("TEST_ENV", ""), args{"TEST_ENV", "default"}, ""},
		{"return default", setup("", ""), args{"TEST_ENV", "default"}, "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			if got := GetEnv(tt.args.env, tt.args.def); got != tt.want {
				t.Errorf("GetEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
