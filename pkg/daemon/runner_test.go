package daemon

import (
	"context"
	"errors"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/metrics"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"
)

var ErrUnexpectedMetricsPublished = errors.New("unexpected metrics")

type mockGenerator struct {
	results []metrics.Metric
	custom  []metrics.Metric
}

func (m mockGenerator) ProduceMetrics(_ context.Context, c chan *metrics.Metric) {
	for _, res := range m.results {
		res := res
		c <- &res
	}
}

func (m mockGenerator) ProduceCustomMetric(_ context.Context, _ config.CustomMetric, c chan *metrics.Metric) {
	for _, res := range m.custom {
		res := res
		c <- &res
	}
}

type mockPublisher struct {
	expected []metrics.Metric
	err      error
}

func (m mockPublisher) PublishMetricsSet(_ context.Context, i []metrics.Metric) error {
	if m.err != nil {
		return m.err
	}
	if !reflect.DeepEqual(m.expected, i) {
		return ErrUnexpectedMetricsPublished
	}
	return nil
}

type mockRecoverableErrorPublisher struct {
	errs []error
	call int
}

func (m *mockRecoverableErrorPublisher) PublishMetricsSet(_ context.Context, _ []metrics.Metric) error {
	m.call++
	if m.call < len(m.errs) {
		return m.errs[m.call]
	}
	return nil
}

func Test_runner_RunOnce(t *testing.T) {
	type fields struct {
		cfg       *config.Config
		consumer  *metrics.Consumer
		generator Generator
		publisher Publisher
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful run no metrics produced",
			fields: fields{
				&config.Config{},
				metrics.NewConsumer(),
				mockGenerator{results: []metrics.Metric{}},
				mockPublisher{expected: []metrics.Metric{}},
			},
			args:    args{context.Background()},
			wantErr: false,
		},
		{
			name: "successful run metrics produced",
			fields: fields{
				&config.Config{},
				metrics.NewConsumer(),
				mockGenerator{results: []metrics.Metric{{Metric: "count", Points: [][]float64{{1608114735, 1}}}}},
				mockPublisher{expected: []metrics.Metric{{Metric: "count", Points: [][]float64{{1608114735, 1}}}}},
			},
			args:    args{context.Background()},
			wantErr: false,
		},
		{
			name: "failed run unrecoverable error",
			fields: fields{
				&config.Config{},
				metrics.NewConsumer(),
				mockGenerator{results: []metrics.Metric{{Metric: "count", Points: [][]float64{{1608114735, 1}}}}},
				mockPublisher{err: metrics.NewUnrecoverableError(errors.New("bad request 400"))},
			},
			args:    args{context.Background()},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Runner{
				cfg:       tt.fields.cfg,
				consumer:  tt.fields.consumer,
				generator: tt.fields.generator,
				publisher: tt.fields.publisher,
			}
			if err := d.RunOnce(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("RunOnce() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_runner_RunUntil(t *testing.T) {
	ctx := func(ctx context.Context, _ context.CancelFunc) context.Context {
		return ctx
	}
	type fields struct {
		cfg       *config.Config
		consumer  *metrics.Consumer
		generator Generator
		publisher Publisher
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "unrecoverable error occurs",
			fields: fields{
				cfg:       &config.Config{MetricInterval: time.Millisecond * 50},
				consumer:  metrics.NewConsumer(),
				generator: mockGenerator{results: []metrics.Metric{{Metric: "row_count", Points: [][]float64{{1608114735, 1}}}}},
				publisher: mockPublisher{err: metrics.NewUnrecoverableError(errors.New("400 bad request"))}},
			args:    args{ctx(context.WithTimeout(context.Background(), time.Millisecond*200))},
			wantErr: true,
		},
		{
			name: "recoverable error occurs",
			fields: fields{
				cfg:       &config.Config{MetricInterval: time.Millisecond * 50},
				consumer:  metrics.NewConsumer(),
				generator: mockGenerator{results: []metrics.Metric{{Metric: "row_count", Points: [][]float64{{1608114735, 1}}}}},
				publisher: &mockRecoverableErrorPublisher{errs: []error{
					metrics.NewRecoverableError(errors.New("429 too many requests")),
				}}},
			args:    args{ctx(context.WithTimeout(context.Background(), time.Millisecond*200))},
			wantErr: false,
		},
		{
			name: "successful publish",
			fields: fields{
				cfg:       &config.Config{MetricInterval: time.Millisecond * 50},
				consumer:  metrics.NewConsumer(),
				generator: mockGenerator{results: []metrics.Metric{{Metric: "row_count", Points: [][]float64{{1608114735, 1}}}}},
				publisher: mockPublisher{expected: []metrics.Metric{{Metric: "row_count", Points: [][]float64{{1608114735, 1}}}}},
			},
			args:    args{ctx(context.WithTimeout(context.Background(), time.Millisecond*200))},
			wantErr: false,
		},
		{
			name: "custom metrics",
			fields: fields{
				cfg: &config.Config{
					MetricInterval: time.Millisecond * 50,
					CustomMetrics: []config.CustomMetric{{
						MetricInterval: time.Millisecond * 50,
					}},
				},
				consumer: metrics.NewConsumer(),
				generator: mockGenerator{
					results: []metrics.Metric{{Metric: "row_count", Points: [][]float64{{1608114735, 1}}}},
					custom:  []metrics.Metric{{Metric: "custom", Points: [][]float64{{1608114736, 500}}}},
				},
				publisher: mockPublisher{expected: []metrics.Metric{
					{Metric: "row_count", Points: [][]float64{{1608114735, 1}}},
					{Metric: "custom", Points: [][]float64{{1608114736, 500}}},
				}},
			},
			args : args{ctx(context.WithTimeout(context.Background(), time.Millisecond*200))},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Runner{
				cfg:       tt.fields.cfg,
				consumer:  tt.fields.consumer,
				generator: tt.fields.generator,
				publisher: tt.fields.publisher,
			}
			if err := d.RunUntil(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("RunUntil() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewRunner(t *testing.T) {
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

	got, err := NewRunner(context.TODO(), &config.Config{})
	if err != nil {
		t.Errorf("NewRunner() error = %v, want %v", err, nil)
		return
	}
	if !reflect.DeepEqual(got.cfg, &config.Config{}) {
		t.Errorf("NewRunner() got.cfg = %+v, want %+v", got.cfg, &config.Config{})
	}
	if !reflect.DeepEqual(got.consumer, metrics.NewConsumer()) {
		t.Errorf("NewRunner() got.consumer = %+v, want %+v", got.consumer, metrics.NewConsumer())
	}
	if !reflect.DeepEqual(got.publisher, metrics.NewDatadogPublisher(&config.Config{})) {
		t.Errorf("NewRunner() got.publisher = %+v, want %+v", got.publisher, metrics.NewDatadogPublisher(&config.Config{}))
	}
}