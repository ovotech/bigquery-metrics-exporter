package config

import (
	"context"
	"errors"
	"github.com/googleapis/gax-go/v2"
	smpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
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
		{"all via env", setup([]string{"DATADOG_API_KEY=abc123", "DATASET_FILTER=bqmetrics:enabled", "GCP_PROJECT_ID=my-project-id", "METRIC_PREFIX=custom.gcp.bigquery.stats", "METRIC_TAGS=env:prod", "METRIC_INTERVAL=2m"}, nil, ""), args{"bqmetricstest"}, &Config{
			DatadogAPIKey:  "abc123",
			DatasetFilter:  "bqmetrics:enabled",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: 2 * time.Minute,
			Profiling:      false,
		}, false},
		{"all via cmd", setup(nil, []string{"--datadog-api-key-file=/tmp/dd.key", "--dataset-filter=bqmetrics:enabled", "--gcp-project-id=my-project-id", "--metric-prefix=custom.gcp.bigquery.stats", "--metric-tags=env:prod", "--metric-interval=2m", "--enable-profiler"}, "abc123"), args{"bqmetricstest"}, &Config{
			DatadogAPIKey:  "abc123",
			DatasetFilter:  "bqmetrics:enabled",
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
		{"default credentials", setup([]string{"DATADOG_API_KEY=abc123", "GOOGLE_APPLICATION_CREDENTIALS=/tmp/dd.key"}, nil, "{\"type\": \"service_account\", \"project_id\": \"my-project-id\"}"), args{"bqmetricstest"}, &Config{
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

func TestNewConfig_configFile(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "config_*.json")
	if err != nil {
		t.Fatalf("error creating temporary file: %s", err)
	}
	defer func() {
		n := f.Name()
		_ = f.Close()
		_ = os.Remove(n)
	}()

	data := []byte("{\"datadog-api-key\": \"abc123\", \"dataset-filter\": \"bqmetrics:enabled\", \"gcp-project-id\": \"my-project-id\", \"metric-prefix\": \"custom.gcp.bigquery.stats\", \"metric-tags\": \"env:prod,team:my-team\", \"metric-interval\": \"2m\"}")
	if _, err = f.Write(data); err != nil {
		t.Fatalf("error when writing test config file: %s", err)
	}

	os.Args = []string{"./bqmetricstest", "--config-file", f.Name()}
	want := &Config{
		DatadogAPIKey:  "abc123",
		DatasetFilter:  "bqmetrics:enabled",
		GcpProject:     "my-project-id",
		MetricPrefix:   "custom.gcp.bigquery.stats",
		MetricTags:     []string{"env:prod", "team:my-team"},
		MetricInterval: 2 * time.Minute,
		Profiling:      false,
	}

	got, err := NewConfig("bqmetricstest")
	if err != nil {
		t.Errorf("NewConfig() error = %v, wantErr false", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("NewConfig() got = %v, want %v", got, want)
	}
}

func TestNewConfig_configFile_invalidFormat(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "config_*.dat")
	if err != nil {
		t.Fatalf("error creating temporary file: %s", err)
	}
	defer func() {
		n := f.Name()
		_ = f.Close()
		_ = os.Remove(n)
	}()

	data := []byte("{\"datadog-api-key\": \"abc123\"}")
	if _, err = f.Write(data); err != nil {
		t.Fatalf("error when writing test config file: %s", err)
	}

	os.Args = []string{"./bqmetricstest", "--config-file", f.Name()}

	_, err = NewConfig("bqmetricstest")
	if err == nil {
		t.Errorf("NewConfig() error = %v, wantErr true", err)
	}
}

func TestNewConfig_configFileWithCustomQueries(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "config_*.json")
	if err != nil {
		t.Fatalf("error creating temporary file: %s", err)
	}
	defer func() {
		n := f.Name()
		_ = f.Close()
		_ = os.Remove(n)
	}()

	data := []byte("{\"datadog-api-key\": \"abc123\", \"gcp-project-id\": \"my-project-id\", \"metric-prefix\": \"custom.gcp.bigquery.stats\", \"metric-tags\": \"env:prod,team:my-team\", \"metric-interval\": \"2m\", \"custom-metrics\": [{\"metric-name\": \"my_metric\", \"metric-tags\": [\"table_id:table\"], \"sql\": \"SELECT COUNT(DISTINCT *) FROM `table`\"}]}")
	if _, err = f.Write(data); err != nil {
		t.Fatalf("error when writing test config file: %s", err)
	}

	os.Args = []string{"./bqmetricstest", "--config-file", f.Name()}
	want := &Config{
		DatadogAPIKey:  "abc123",
		GcpProject:     "my-project-id",
		MetricPrefix:   "custom.gcp.bigquery.stats",
		MetricTags:     []string{"env:prod", "team:my-team"},
		MetricInterval: 2 * time.Minute,
		Profiling:      false,
		CustomMetrics: []CustomMetric{{
			MetricName:     "my_metric",
			MetricTags:     []string{"table_id:table"},
			MetricInterval: 2 * time.Minute,
			SQL:            "SELECT COUNT(DISTINCT *) FROM `table`",
		}},
	}

	got, err := NewConfig("bqmetricstest")
	if err != nil {
		t.Errorf("NewConfig() error = %v, wantErr false", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("NewConfig() got = %v, want %v", got, want)
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
		{"custom metrics okay", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(30000),
			CustomMetrics: []CustomMetric{{
				MetricName:     "my_custom_metric",
				MetricTags:     []string{"table_id:my-table"},
				MetricInterval: time.Duration(36000000),
				SQL:            "SELECT COUNT(DISTINCT `my-column`) FROM `my-dataset.my-table`",
			}},
		}}, false},
		{"custom metrics missing name", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(30000),
			CustomMetrics: []CustomMetric{{
				MetricInterval: time.Duration(36000000),
				SQL:            "SELECT COUNT(DISTINCT `my-column`) FROM `my-dataset.my-table`",
			}},
		}}, true},
		{"custom metrics missing sql", args{&Config{
			DatadogAPIKey:  "abc123",
			GcpProject:     "my-project-id",
			MetricPrefix:   "custom.gcp.bigquery.stats",
			MetricTags:     []string{"env:prod"},
			MetricInterval: time.Duration(30000),
			CustomMetrics: []CustomMetric{{
				MetricName:     "my_custom_metric",
				MetricInterval: time.Duration(36000000),
			}},
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

func Test_getDefaultProjectID_missingAuthentication(t *testing.T) {
	want := ""

	os.Clearenv()
	_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/tmp/non_existing_credentials.json")

	got, err := getDefaultProjectID()
	if err == nil {
		t.Errorf("getDefaultProjectID() error = %v, wantErr %v", err, true)
		return
	}
	if got != want {
		t.Errorf("getDefaultProjectID() got = %v, want %v", got, want)
	}
}

func Test_getDefaultProjectID_presentAuthentication(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "config_*.json")
	if err != nil {
		t.Fatalf("error creating temporary file: %s", err)
	}
	defer func() {
		n := f.Name()
		_ = f.Close()
		_ = os.Remove(n)
	}()

	data := []byte("{\"type\": \"service_account\", \"project_id\": \"my-project-id\"}")
	if _, err = f.Write(data); err != nil {
		t.Fatalf("error when writing test config file: %s", err)
	}

	os.Clearenv()
	_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", f.Name())

	want := "my-project-id"

	got, err := getDefaultProjectID()
	if err != nil {
		t.Errorf("getDefaultProjectID() error = %v, wantErr %v", err, false)
		return
	}
	if got != want {
		t.Errorf("getDefaultProjectID() got = %v, want %v", got, want)
	}
}

func TestNormaliseConfig(t *testing.T) {
	tests := []struct {
		name string
		arg  *Config
		want *Config
	}{
		{
			"no custom metrics",
			&Config{},
			&Config{},
		},
		{
			"custom metric missing interval",
			&Config{MetricInterval: time.Second * 10, CustomMetrics: []CustomMetric{{MetricName: "my-metric"}}},
			&Config{MetricInterval: time.Second * 10, CustomMetrics: []CustomMetric{{MetricName: "my-metric", MetricInterval: time.Second * 10}}},
		},
		{
			"custom metric with interval",
			&Config{MetricInterval: time.Second * 5, CustomMetrics: []CustomMetric{{MetricName: "my-metric", MetricInterval: time.Second * 10}}},
			&Config{MetricInterval: time.Second * 5, CustomMetrics: []CustomMetric{{MetricName: "my-metric", MetricInterval: time.Second * 10}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NormaliseConfig(tt.arg)
			if !reflect.DeepEqual(tt.want, tt.arg) {
				t.Errorf("NormaliseConfig() got = %v, want = %v", tt.arg, tt.want)
			}
		})
	}
}

type mockSecretManagerClient struct {
	payload []byte
	err     error
}

func (m mockSecretManagerClient) AccessSecretVersion(_ context.Context, req *smpb.AccessSecretVersionRequest, _ ...gax.CallOption) (*smpb.AccessSecretVersionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}

	return &smpb.AccessSecretVersionResponse{
		Name:    req.Name,
		Payload: &smpb.SecretPayload{Data: m.payload},
	}, nil
}

func Test_getValueFromSecretManager(t *testing.T) {
	type args struct {
		id     string
		client secretManagerClient
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"secret exists",
			args{id: "123", client: mockSecretManagerClient{[]byte("my-secret-value"), nil}},
			"my-secret-value",
			false,
		},
		{
			"secret doesnt exist",
			args{id: "456", client: mockSecretManagerClient{nil, errors.New("secret not found")}},
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getValueFromSecretManager(tt.args.id, tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("getValueFromSecretManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getValueFromSecretManager() got = %v, want %v", got, tt.want)
			}
		})
	}
}
