package metrics

import (
	"context"
	"errors"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"reflect"
	"testing"
	"time"
)

var ErrUnexpectedMetricsPublished = errors.New("unexpected metrics")

type mockPublisher struct {
	expected []Metric
	err      error
}

func (m mockPublisher) PublishMetricsSet(_ context.Context, i []Metric) error {
	if m.err != nil {
		return m.err
	}
	if !reflect.DeepEqual(m.expected, i) {
		return ErrUnexpectedMetricsPublished
	}
	return nil
}

func TestMetric_Id(t *testing.T) {
	type fields struct {
		Metric string
		Tags   []string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"no tags", fields{"row_count", nil}, "row_count"},
		{"one tag", fields{"row_count", []string{"env:prod"}}, "row_count;env:prod"},
		{"multi tags", fields{"row_count", []string{"env:prod", "level:warning"}}, "row_count;env:prod;level:warning"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metric{
				Metric: tt.fields.Metric,
				Tags:   tt.fields.Tags,
			}
			if got := m.ID(); got != tt.want {
				t.Errorf("ID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetric_append(t *testing.T) {
	type fields struct {
		Interval uint64
		Metric   string
		Points   [][]float64
		Tags     []string
		Type     string
	}
	type args struct {
		r Reading
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Metric
	}{
		{
			name: "append reading to empty",
			args: args{Reading{time.Unix(1600, 0), 1}},
			want: &Metric{Points: [][]float64{{1600, 1}}},
		},
		{
			name:   "append reading to existing",
			fields: fields{Points: [][]float64{{1600, 1}}},
			args:   args{Reading{time.Unix(1660, 0), 2}},
			want:   &Metric{Points: [][]float64{{1600, 1}, {1660, 2}}},
		},
		{
			name:   "append reading with existing timestamp",
			fields: fields{Points: [][]float64{{1600, 1}}},
			args:   args{Reading{time.Unix(1600, 0), 2}},
			want:   &Metric{Points: [][]float64{{1600, 2}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metric{
				Interval: tt.fields.Interval,
				Metric:   tt.fields.Metric,
				Points:   tt.fields.Points,
				Tags:     tt.fields.Tags,
				Type:     tt.fields.Type,
			}
			m.append(tt.args.r)
			if !reflect.DeepEqual(tt.want, m) {
				t.Errorf("metric after append() = %v, want %v", m, tt.want)
			}
		})
	}
}

func TestMetric_mergePoints(t *testing.T) {
	type fields struct {
		Interval uint64
		Metric   string
		Points   [][]float64
		Tags     []string
		Type     string
	}
	type args struct {
		o *Metric
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Metric
	}{
		{
			name: "merge empty into empty",
			args: args{&Metric{}},
			want: &Metric{},
		},
		{
			name:   "merge empty into existing metric",
			fields: fields{Points: [][]float64{{1600, 1}}},
			args:   args{&Metric{}},
			want:   &Metric{Points: [][]float64{{1600, 1}}},
		},
		{
			name: "merge existing into empty",
			args: args{&Metric{Points: [][]float64{{1600, 1}}}},
			want: &Metric{Points: [][]float64{{1600, 1}}},
		},
		{
			name:   "merge existing into existing",
			fields: fields{Points: [][]float64{{1600, 1}}},
			args:   args{&Metric{Points: [][]float64{{1660, 2}}}},
			want:   &Metric{Points: [][]float64{{1600, 1}, {1660, 2}}},
		},
		{
			name:   "merge existing with new values into existing",
			fields: fields{Points: [][]float64{{1600, 1}}},
			args:   args{&Metric{Points: [][]float64{{1600, 2}}}},
			want:   &Metric{Points: [][]float64{{1600, 2}}},
		},
		{
			name:   "merge multiple into existing",
			fields: fields{Points: [][]float64{{1600, 1}}},
			args:   args{&Metric{Points: [][]float64{{1660, 2}, {1720, 3}}}},
			want:   &Metric{Points: [][]float64{{1600, 1}, {1660, 2}, {1720, 3}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metric{
				Interval: tt.fields.Interval,
				Metric:   tt.fields.Metric,
				Points:   tt.fields.Points,
				Tags:     tt.fields.Tags,
				Type:     tt.fields.Type,
			}
			m.mergePoints(tt.args.o)
			if !reflect.DeepEqual(tt.want, m) {
				t.Errorf("metric after mergePoints() = %v, want %v", m, tt.want)
			}
		})
	}
}

func Test_getFullMetricName(t *testing.T) {
	type args struct {
		prefix string
		metric string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"no prefix", args{metric: "row_count"}, "row_count"},
		{"with prefix", args{"custom.table", "row_count"}, "custom.table.row_count"},
		{"with suffix dot", args{"custom.table.", "row_count"}, "custom.table.row_count"},
		{"with prefix dot", args{"custom.table", ".row_count"}, "custom.table.row_count"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getFullMetricName(tt.args.prefix, tt.args.metric); got != tt.want {
				t.Errorf("getFullMetricName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_createPointMap(t *testing.T) {
	mkp := func(in float64) *float64 {
		return &in
	}
	type args struct {
		m *Metric
	}
	tests := []struct {
		name string
		args args
		want pointmap
	}{
		{"simple point map", args{&Metric{Points: [][]float64{{1600, 1}}}}, pointmap{1600: mkp(1)}},
		{"multiple points map", args{&Metric{Points: [][]float64{{1600, 1}, {1720, 3}}}}, pointmap{1600: mkp(1), 1720: mkp(3)}},
		{"ignore invalid points", args{&Metric{Points: [][]float64{{1600}, {1720, 3, 1}}}}, pointmap{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createPointmap(tt.args.m); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createPointMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProducer_Produce(t *testing.T) {
	type fields struct {
		config *config.Config
	}
	type args struct {
		metric string
		read   Reading
		tags   []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Metric
	}{
		{
			name:   "simple metric",
			fields: fields{&config.Config{}},
			args:   args{"row_count", Reading{time.Unix(1600, 0), 1}, nil},
			want:   &Metric{Metric: "row_count", Points: [][]float64{{1600, 1}}, Type: TypeGauge},
		},
		{
			name:   "metric includes prefix",
			fields: fields{&config.Config{MetricPrefix: "custom.tables"}},
			args:   args{"row_count", Reading{time.Unix(1600, 0), 1}, nil},
			want:   &Metric{Metric: "custom.tables.row_count", Points: [][]float64{{1600, 1}}, Type: TypeGauge},
		},
		{
			name:   "metric includes interval",
			fields: fields{&config.Config{MetricInterval: 30 * time.Second}},
			args:   args{"row_count", Reading{time.Unix(1600, 0), 1}, nil},
			want:   &Metric{Interval: 30, Metric: "row_count", Points: [][]float64{{1600, 1}}, Type: TypeGauge},
		},
		{
			name:   "metric includes default tags",
			fields: fields{&config.Config{MetricTags: []string{"env:prod"}}},
			args:   args{"row_count", Reading{time.Unix(1600, 0), 1}, nil},
			want:   &Metric{Metric: "row_count", Points: [][]float64{{1600, 1}}, Tags: []string{"env:prod"}, Type: TypeGauge},
		},
		{
			name:   "metric includes metric tags",
			fields: fields{&config.Config{}},
			args:   args{"row_count", Reading{time.Unix(1600, 0), 1}, []string{"table:my_cool_table"}},
			want:   &Metric{Metric: "row_count", Points: [][]float64{{1600, 1}}, Tags: []string{"table:my_cool_table"}, Type: TypeGauge},
		},
		{
			name:   "metric merges and sorts metric and default tags",
			fields: fields{&config.Config{MetricTags: []string{"env:prod"}}},
			args:   args{"row_count", Reading{time.Unix(1600, 0), 1}, []string{"dataset:secure", "table:my_cool_table"}},
			want:   &Metric{Metric: "row_count", Points: [][]float64{{1600, 1}}, Tags: []string{"dataset:secure", "env:prod", "table:my_cool_table"}, Type: TypeGauge},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Producer{
				config: tt.fields.config,
			}
			if got := p.Produce(tt.args.metric, tt.args.read, tt.args.tags); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Produce() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsumer_Run(t *testing.T) {
	c := NewConsumer()
	receiver := c.Run()

	if len(c.metrics) != 0 {
		t.Errorf("len(c.metrics) = %v, want %v", len(c.metrics), 0)
	}

	receiver <- &Metric{Metric: "row_count", Points: [][]float64{{1600, 1}}}

	c.mx.Lock()
	defer c.mx.Unlock()
	if len(c.metrics) != 1 {
		t.Errorf("len(c.metrics) = %v, want %v", len(c.metrics), 1)
	}
}

func TestConsumer_Flush(t *testing.T) {
	c := NewConsumer()
	c.consume(&Metric{Metric: "row_count", Points: [][]float64{{1600, 1}}})

	if len(c.metrics) != 1 {
		t.Errorf("len(c.metrics) = %v, want %v", len(c.metrics), 1)
	}

	want := []Metric{{Metric: "row_count", Points: [][]float64{{1600, 1}}}}
	if got := c.Flush(); !reflect.DeepEqual(got, want) {
		t.Errorf("Flush() = %v, want %v", got, want)
	}

	if len(c.metrics) != 0 {
		t.Errorf("len(c.metrics) = %v, want %v", len(c.metrics), 0)
	}
}

func TestConsumer_PublishTo(t *testing.T) {
	type fields struct {
		metrics map[string]*Metric
	}
	type args struct {
		ctx context.Context
		pub publisher
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "successful publish",
			fields:  fields{map[string]*Metric{"row_count": {Metric: "row_count", Points: [][]float64{{1600, 1}}}}},
			args:    args{context.Background(), mockPublisher{expected: []Metric{{Metric: "row_count", Points: [][]float64{{1600, 1}}}}}},
			wantErr: false,
		},
		{
			name:    "publish no metrics",
			fields:  fields{map[string]*Metric{}},
			args:    args{context.Background(), mockPublisher{}},
			wantErr: false,
		},
		{
			name:    "publish failure",
			fields:  fields{map[string]*Metric{"row_count": {Metric: "row_count", Points: [][]float64{{1600, 1}}}}},
			args:    args{context.Background(), mockPublisher{err: NewUnrecoverableError(errors.New("400 bad request"))}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Consumer{
				metrics: tt.fields.metrics,
			}
			if err := c.PublishTo(tt.args.ctx, tt.args.pub); (err != nil) != tt.wantErr {
				t.Errorf("PublishTo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConsumer_PublishTo_SuccessFlushesMetrics(t *testing.T) {
	c := &Consumer{
		metrics: map[string]*Metric{"row_count": {Metric: "row_count", Points: [][]float64{{1600, 1}}}},
	}

	if len(c.metrics) != 1 {
		t.Errorf("len(c.metrics) = %v, want %v", len(c.metrics), 1)
	}

	pub := mockPublisher{expected: []Metric{{Metric: "row_count", Points: [][]float64{{1600, 1}}}}}
	if err := c.PublishTo(context.Background(), pub); (err != nil) != false {
		t.Errorf("PublishTo() error = %v, wantErr %v", err, false)
	}

	if len(c.metrics) != 0 {
		t.Errorf("len(c.metrics) = %v, want %v", len(c.metrics), 0)
	}
}

func TestConsumer_PublishTo_UnrecoverableFailureFlushesMetrics(t *testing.T) {
	c := &Consumer{
		metrics: map[string]*Metric{"row_count": {Metric: "row_count", Points: [][]float64{{1600, 1}}}},
	}

	if len(c.metrics) != 1 {
		t.Errorf("len(c.metrics) = %v, want %v", len(c.metrics), 1)
	}

	pub := mockPublisher{err: NewUnrecoverableError(errors.New("400 bad request"))}
	if err := c.PublishTo(context.Background(), pub); (err != nil) != true {
		t.Errorf("PublishTo() error = %v, wantErr %v", err, true)
	}

	if len(c.metrics) != 0 {
		t.Errorf("len(c.metrics) = %v, want %v", len(c.metrics), 0)
	}
}

func TestConsumer_PublishTo_RecoverableFailureKeepsMetrics(t *testing.T) {
	c := &Consumer{
		metrics: map[string]*Metric{"row_count": {Metric: "row_count", Points: [][]float64{{1600, 1}}}},
	}

	if len(c.metrics) != 1 {
		t.Errorf("len(c.metrics) = %v, want %v", len(c.metrics), 1)
	}

	pub := mockPublisher{err: NewRecoverableError(errors.New("429 too many requests"))}
	if err := c.PublishTo(context.Background(), pub); (err != nil) != true {
		t.Errorf("PublishTo() error = %v, wantErr %v", err, true)
	}

	if len(c.metrics) != 1 {
		t.Errorf("len(c.metrics) = %v, want %v", len(c.metrics), 1)
	}
}

func TestNewReadingFrom(t *testing.T) {
	now := time.Now()
	type args struct {
		val interface{}
		at  time.Time
	}
	tests := []struct {
		name    string
		args    args
		want    Reading
		wantErr bool
	}{
		{"float64", args{float64(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"float32", args{float32(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"int64", args{int64(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"int32", args{int32(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"int16", args{int16(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"int8", args{int8(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"int", args{int(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"uint64", args{uint64(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"uint32", args{uint32(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"uint16", args{uint16(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"uint8", args{uint8(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"uint", args{uint(10), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"true", args{true, now}, Reading{Value: 1.0, Timestamp: now}, false},
		{"false", args{false, now}, Reading{Value: 0.0, Timestamp: now}, false},
		{"time", args{time.Unix(10, 0), now}, Reading{Value: 10.0, Timestamp: now}, false},
		{"string", args{"10", now}, Reading{}, true},
		{"bytes", args{[]byte("10"), now}, Reading{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewReadingFrom(tt.args.val, tt.args.at)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewReadingFrom() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewReadingFrom() got = %v, want %v", got, tt.want)
			}
		})
	}
}
