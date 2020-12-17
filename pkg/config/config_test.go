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
		{"all via env", setup([]string{"DATADOG_API_KEY=abc123", "GCP_PROJECT_ID=ovo-project-id", "METRIC_PREFIX=custom.gcp.bigquery.stats", "METRIC_TAGS=env:prod", "METRIC_INTERVAL=2m"}, nil, ""), args{"bqmetricstest"}, &Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "ovo-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: 2 * time.Minute,
		}, false},
		{"all via cmd", setup(nil, []string{"--datadog-api-key-file=/tmp/dd.key", "--gcp-project-id=ovo-project-id", "--metric-prefix=custom.gcp.bigquery.stats", "--metric-tags=env:prod", "--metric-interval=2m"}, "abc123"), args{"bqmetricstest"}, &Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "ovo-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: 2 * time.Minute,
		}, false},
		{"mixture of sources", setup([]string{"DATADOG_API_KEY=abc123", "GCP_PROJECT_ID=ovo-project-id"}, []string{"--metric-prefix=custom.gcp.bigquery.stats", "--metric-tags=env:prod", "--metric-interval=2m"}, ""), args{"bqmetricstest"}, &Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "ovo-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: 2 * time.Minute,
		}, false},
		{"minimum required config", setup([]string{"DATADOG_API_KEY=abc123", "GCP_PROJECT_ID=ovo-project-id"}, nil, ""), args{"bqmetricstest"}, &Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "ovo-project-id",
			MetricPrefix:   DefaultMetricPrefix,
			MetricTags:     nil,
			MetricInterval: 30 * time.Second,
		}, false},
		{"unreadable key file", setup([]string{"DATADOG_API_KEY_FILE=/tmp/not-found.key", "GCP_PROJECT_ID=ovo-project-id"}, nil, "abc123"), args{"bqmetricstest"}, nil, true},
		{"unparseable interval", setup([]string{"DATADOG_API_KEY_FILE=/tmp/dd.key", "GCP_PROJECT_ID=ovo-project-id"}, []string{"--metric-interval=notaduration"}, "abc123"), args{"bqmetricstest"}, nil, true},
		{"missing key", setup([]string{"GCP_PROJECT_ID=ovo-project-id"}, nil, ""), args{"bqmetricstest"}, nil, true},
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
			GcpProject:     "ovo-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(30000),
		}}, false},
		{"missing datadog api key", args{&Config{
			DatadogAPIKey:  "",
			GcpProject:     "ovo-project-id",
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
			GcpProject:     "ovo-project-id",
			MetricPrefix:   "",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(30000),
		}}, true},
		{"missing metric tags", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "ovo-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     nil,
			MetricInterval: time.Duration(30000),
		}}, false},
		{"missing metric interval", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "ovo-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(0),
		}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateConfig(tt.args.c); (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_argsFromCommandLine(t *testing.T) {
	setup := func(cmd, val string) func() {
		return func() {
			if cmd == "" {
				os.Args = []string{"./bqmetricstest"}
				return
			}

			os.Args = []string{"./bqmetricstest", cmd, val}
		}
	}
	type args struct {
		t string
	}
	tests := []struct {
		name  string
		setup func()
		args  args
		want  arguments
	}{
		{"datadog-api-key-file", setup("--datadog-api-key-file", "/tmp/dd.key"), args{"bqmetricstest"}, arguments{datadogAPIKeyFile: "/tmp/dd.key"}},
		{"datadog-api-key-file empty", setup("", ""), args{"bqmetricstest"}, arguments{datadogAPIKeyFile: ""}},
		{"gcp-project-id", setup("--gcp-project-id", "ovo-project-one"), args{"bqmetricstest"}, arguments{projectID: "ovo-project-one"}},
		{"gcp-project-id empty", setup("", ""), args{"bqmetricstest"}, arguments{projectID: ""}},
		{"metric-prefix", setup("--metric-prefix", "custom.gcp.bigquery.stats"), args{"bqmetricstest"}, arguments{metricPrefix: "custom.gcp.bigquery.stats"}},
		{"metric-prefix empty", setup("", ""), args{"bqmetricstest"}, arguments{metricPrefix: ""}},
		{"metric-interval", setup("--metric-interval", "2m"), args{"bqmetricstest"}, arguments{metricInterval: "2m"}},
		{"metric-interval empty", setup("", ""), args{"bqmetricstest"}, arguments{metricInterval: ""}},
		{"metric-tags", setup("--metric-tags", "env:prod"), args{"bqmetricstest"}, arguments{metricTags: "env:prod"}},
		{"metric-tags empty", setup("", ""), args{"bqmetricstest"}, arguments{metricTags: ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			if got := argsFromCommandLine(tt.args.t); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("argsFromCommandLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_argsFromEnv(t *testing.T) {
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
	tests := []struct {
		name  string
		setup func()
		want  arguments
	}{
		{"datadog_api_key", setup("DATADOG_API_KEY", "abc123"), arguments{datadogAPIKey: "abc123"}},
		{"empty datadog_api_key", setup("", ""), arguments{datadogAPIKey: ""}},
		{"datadog_api_key_file", setup("DATADOG_API_KEY_FILE", "/tmp/dd.key"), arguments{datadogAPIKeyFile: "/tmp/dd.key"}},
		{"empty datadog_api_key_file", setup("", ""), arguments{datadogAPIKey: ""}},
		{"gcp_project_id", setup("GCP_PROJECT_ID", "ovo-project-one"), arguments{projectID: "ovo-project-one"}},
		{"empty gcp_project_id", setup("", ""), arguments{projectID: ""}},
		{"metric_prefix", setup("METRIC_PREFIX", "custom.gcp.bigquery.stats"), arguments{metricPrefix: "custom.gcp.bigquery.stats"}},
		{"empty metric_prefix", setup("", ""), arguments{metricPrefix: ""}},
		{"metric_interval", setup("METRIC_INTERVAL", "2m"), arguments{metricInterval: "2m"}},
		{"empty metric_interval", setup("", ""), arguments{metricInterval: ""}},
		{"metric_tags", setup("METRIC_TAGS", "env:prod"), arguments{metricTags: "env:prod"}},
		{"empty metric_tags", setup("", ""), arguments{metricTags: ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			if got := argsFromEnv(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("argsFromEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_coalesce(t *testing.T) {
	type args struct {
		vals []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"no params", args{[]string{}}, ""},
		{"single param", args{[]string{"param1"}}, "param1"},
		{"two params", args{[]string{"param1", "param2"}}, "param1"},
		{"empty param", args{[]string{""}}, ""},
		{"empty param 1", args{[]string{"", "param2"}}, "param2"},
		{"empty param 2", args{[]string{"param1", "param2"}}, "param1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := coalesce(tt.args.vals...); got != tt.want {
				t.Errorf("coalesce() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseTagString(t *testing.T) {
	type args struct {
		t string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"single tag", args{"prod"}, []string{"prod"}},
		{"kv pair", args{"env:prod"}, []string{"env:prod"}},
		{"multi single tags", args{"prod,warning"}, []string{"prod", "warning"}},
		{"multi kv pairs", args{"env:prod,level:warning"}, []string{"env:prod", "level:warning"}},
		{"tag and kv pair", args{"prod,level:warning"}, []string{"prod", "level:warning"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTagString(tt.args.t); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTagString() = %v, want %v", got, tt.want)
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
