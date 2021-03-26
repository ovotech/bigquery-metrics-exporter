package sources

import (
	"cloud.google.com/go/bigquery"
	"context"
	bq "github.com/googleapis/google-cloud-go-testing/bigquery/bqiface"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/metrics"
	"google.golang.org/api/iterator"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func Test_iterateTables(t *testing.T) {
	ds := newMockDataset("my-dataset", "my-project", []mockTable{
		newMockTableDefaults("table-1"),
		newMockTableDefaults("table-2"),
		newMockTableDefaults("table-3"),
	})
	out := iterateTables(context.TODO(), ds)

	got := make([]string, 0)
	for tbl := range out {
		got = append(got, tbl.TableID())
	}

	want := []string{"table-1", "table-2", "table-3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("iterateTables() got %v, want %v", got, want)
	}
}

func Test_iterateDatasets(t *testing.T) {
	cl := newMockClient("my-project", []mockDataset{
		newMockDatasetDefaults("dataset-1"),
		newMockDatasetDefaults("dataset-2"),
	})
	out := iterateDatasets(context.TODO(), cl, "")

	got := make([]string, 0)
	for ds := range out {
		got = append(got, ds.DatasetID())
	}

	want := []string{"dataset-1", "dataset-2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("iterateDatasets() got %v, want %v", got, want)
	}
}

func TestGenerator_outputTableLevelMetrics(t *testing.T) {
	type args struct {
		t bq.Table
	}
	tests := []struct {
		name string
		args args
		want []*metrics.Metric
	}{
		{
			"regular table metrics",
			args{newMockTable("my-table", "my-dataset", "my-project", bigquery.RegularTable, time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC), 0)},
			[]*metrics.Metric{
				{
					Interval: 0,
					Metric:   "table.row_count",
					Points:   [][]float64{{float64(time.Now().Unix()), 0}},
					Tags:     []string{"dataset_id:my-dataset", "project_id:my-project", "table_id:my-table"},
					Type:     metrics.TypeGauge,
				},
				{
					Interval: 0,
					Metric:   "table.last_modified_time",
					Points:   [][]float64{{float64(time.Now().Unix()), 1577880000}},
					Tags:     []string{"dataset_id:my-dataset", "project_id:my-project", "table_id:my-table"},
					Type:     metrics.TypeGauge,
				},
				{
					Interval: 0,
					Metric:   "table.last_modified",
					Points:   [][]float64{{float64(time.Now().Unix()), float64(time.Now().Unix()) - 1577880000}},
					Tags:     []string{"dataset_id:my-dataset", "project_id:my-project", "table_id:my-table"},
					Type:     metrics.TypeGauge,
				},
			},
		},
		{
			"view table metrics",
			args{newMockTable("my-view", "my-dataset", "my-project", bigquery.ViewTable, time.Now(), 0)},
			[]*metrics.Metric{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := Generator{
				cfg:      &config.Config{},
				client:   mockClient{},
				producer: metrics.NewProducer(&config.Config{}),
			}

			out := make(chan *metrics.Metric, 100)
			got := make([]*metrics.Metric, 0)

			wg := &sync.WaitGroup{}
			wg.Add(1)
			go g.outputTableLevelMetrics(context.TODO(), tt.args.t, out, wg)
			wg.Wait()

			close(out)
			for met := range out {
				got = append(got, met)
			}

			if len(got) != len(tt.want) {
				t.Errorf("outputTableLevelMetrics() got len = %v, want len = %v", len(got), len(tt.want))
				return
			}

			for i := range got {
				if !compareMetrics(got[i], tt.want[i]) {
					t.Errorf("outputTableLevelMetrics() got metric = %v, want metric = %v", *got[i], *tt.want[i])
				}
			}
		})
	}
}

func compareMetrics(got, want *metrics.Metric) bool {
	if got.ID() != want.ID() {
		return false
	}

	if got.Type != want.Type {
		return false
	}

	if got.Interval != want.Interval {
		return false
	}

	if len(got.Points) != len(want.Points) {
		return false
	}

	for i := range got.Points {
		// Dont compare timestamps of metric readings
		if got.Points[i][1] != want.Points[i][1] {
			return false
		}
	}

	return true
}

func TestGenerator_runSQLQuery(t *testing.T) {
	g := Generator{
		cfg:      &config.Config{},
		client:   mockClient{},
		producer: metrics.NewProducer(&config.Config{}),
	}

	got, err := g.runSQLQuery(context.TODO(), "SELECT * FROM `my_dataset.my_table`")
	if err != nil {
		t.Errorf("runSQLQuery() err = %v, want = %v", err, nil)
		return
	}

	want := &mockRowIterator{rows: []map[string]bigquery.Value{}, idx: 0}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("runSQLQuery() got = %v, want = %v", got, want)
	}
}

func TestGenerator_produceCustomMetrics(t *testing.T) {
	g := Generator{
		cfg: &config.Config{},
		client: mockClient{
			query: &mockQuery{
				job: &mockJob{
					rows: &mockRowIterator{
						rows: []map[string]bigquery.Value{{"count": 100}},
					},
				},
			},
		},
		producer: metrics.NewProducer(&config.Config{}),
	}

	cm := config.CustomMetric{
		MetricName:     "row_count",
		MetricTags:     []string{"table_id:my-view"},
		MetricInterval: time.Second * 3600,
		SQL:            "SELECT COUNT(*) AS `count` FROM `my-view`",
	}

	collector := make(chan *metrics.Metric, 1)
	g.ProduceCustomMetric(context.TODO(), cm, collector)
	close(collector)

	got := <-collector
	want := &metrics.Metric{
		Interval: 0,
		Metric:   "custom_metric.row_count",
		Points:   [][]float64{{float64(time.Now().Unix()), 100.0}},
		Tags:     []string{"column_id:count", "table_id:my-view"},
		Type:     metrics.TypeGauge,
	}
	if !compareMetrics(want, got) {
		t.Errorf("ProduceCustomMetric() got = %v, want = %v", got, want)
	}
}

func TestGenerator_produceCustomMetrics_noMetricsWhenNoResults(t *testing.T) {
	g := Generator{
		cfg:      &config.Config{},
		client:   mockClient{},
		producer: metrics.NewProducer(&config.Config{}),
	}

	cm := config.CustomMetric{
		MetricName:     "row_count",
		MetricTags:     []string{"table_id:my-view"},
		MetricInterval: time.Second * 3600,
		SQL:            "SELECT COUNT(*) AS `count` FROM `my-view` WHERE 1 = 0",
	}

	collector := make(chan *metrics.Metric, 100)
	g.ProduceCustomMetric(context.TODO(), cm, collector)
	close(collector)

	got := make([]*metrics.Metric, 0)
	for met := range collector {
		got = append(got, met)
	}

	want := 0
	if len(got) != want {
		t.Errorf("ProduceCustomMetric() got len = %v, want len = %v", got, want)
	}
}

func TestGenerator_produceCustomMetrics_produceMetricsWhenZeroResult(t *testing.T) {
	g := Generator{
		cfg: &config.Config{},
		client: mockClient{
			query: &mockQuery{
				job: &mockJob{
					rows: &mockRowIterator{
						rows: []map[string]bigquery.Value{{"count": 0}},
					},
				},
			},
		},
		producer: metrics.NewProducer(&config.Config{}),
	}

	cm := config.CustomMetric{
		MetricName:     "row_count",
		MetricTags:     []string{"table_id:my-unused-view"},
		MetricInterval: time.Second * 3600,
		SQL:            "SELECT COUNT(*) AS `count` FROM `my-unused-view`",
	}

	collector := make(chan *metrics.Metric, 1)
	g.ProduceCustomMetric(context.TODO(), cm, collector)
	close(collector)

	got := <-collector
	want := &metrics.Metric{
		Interval: 0,
		Metric:   "custom_metric.row_count",
		Points:   [][]float64{{float64(time.Now().Unix()), 0.0}},
		Tags:     []string{"column_id:count", "table_id:my-unused-view"},
		Type:     metrics.TypeGauge,
	}
	if !compareMetrics(want, got) {
		t.Errorf("ProduceCustomMetric() got = %v, want = %v", got, want)
	}
}

type mockClient struct {
	bq.Client
	proj     string
	datasets []mockDataset
	query    *mockQuery
}

func (m mockClient) Datasets(_ context.Context) bq.DatasetIterator {
	return &mockDatasetIterator{
		filter:   "",
		datasets: m.datasets,
		idx:      0,
	}
}

func (m mockClient) Query(sql string) bq.Query {
	if m.query != nil {
		return m.query
	}

	cfg := bigquery.QueryConfig{}
	cfg.Q = sql
	return &mockQuery{
		cfg: bq.QueryConfig{QueryConfig: cfg},
	}
}

func newMockClient(proj string, datasets []mockDataset) mockClient {
	for i := range datasets {
		datasets[i].proj = proj
	}
	return mockClient{
		proj:     proj,
		datasets: datasets,
	}
}

type mockDatasetIterator struct {
	bq.DatasetIterator
	filter   string
	datasets []mockDataset
	idx      int
}

func (m *mockDatasetIterator) SetFilter(s string) {
	m.filter = s
}

func (m *mockDatasetIterator) Next() (bq.Dataset, error) {
	if m.idx >= len(m.datasets) {
		return nil, iterator.Done
	}

	t := m.datasets[m.idx]
	m.idx++
	return t, nil
}

func (m mockDatasetIterator) PageInfo() *iterator.PageInfo {
	return &iterator.PageInfo{}
}

type mockDataset struct {
	bq.Dataset
	dataset string
	proj    string
	tables  []mockTable
}

func newMockDatasetDefaults(dataset string) mockDataset {
	return newMockDataset(dataset, "", []mockTable{})
}

func newMockDataset(dataset, proj string, tables []mockTable) mockDataset {
	for i := range tables {
		tables[i].dataset = dataset
		tables[i].project = proj
	}
	return mockDataset{dataset: dataset, proj: proj, tables: tables}
}

func (m mockDataset) ProjectID() string {
	return m.proj
}

func (m mockDataset) DatasetID() string {
	return m.dataset
}

func (m mockDataset) Table(s string) bq.Table {
	for _, t := range m.tables {
		if t.TableID() == s {
			return t
		}
	}

	return nil
}

func (m mockDataset) Tables(_ context.Context) bq.TableIterator {
	return &mockTableIterator{
		tables: m.tables,
		idx:    0,
	}
}

type mockQuery struct {
	bq.Query
	cfg bq.QueryConfig
	job bq.Job
}

func (m *mockQuery) SetQueryConfig(cfg bq.QueryConfig) {
	m.cfg = cfg
}

func (m *mockQuery) Run(_ context.Context) (bq.Job, error) {
	if m.job != nil {
		return m.job, nil
	}

	return &mockJob{}, nil
}

type mockJob struct {
	bq.Job
	rows bq.RowIterator
}

func (m *mockJob) Read(_ context.Context) (bq.RowIterator, error) {
	if m.rows != nil {
		return m.rows, nil
	}

	return &mockRowIterator{
		rows: []map[string]bigquery.Value{},
		idx:  0,
	}, nil
}

type mockRowIterator struct {
	bq.RowIterator
	rows []map[string]bigquery.Value
	idx  int
}

func (m *mockRowIterator) TotalRows() uint64 {
	return uint64(len(m.rows))
}

func (m *mockRowIterator) Next(out interface{}) error {
	if m.idx >= len(m.rows) {
		return iterator.Done
	}

	vm := out.(*map[string]bigquery.Value)
	*vm = m.rows[m.idx]
	m.idx++
	out = vm

	return nil
}

type mockTableIterator struct {
	bq.TableIterator
	tables []mockTable
	idx    int
}

func (m *mockTableIterator) Next() (bq.Table, error) {
	if m.idx >= len(m.tables) {
		return nil, iterator.Done
	}

	t := m.tables[m.idx]
	m.idx++
	return t, nil
}

func (m mockTableIterator) PageInfo() *iterator.PageInfo {
	return &iterator.PageInfo{}
}

type mockTable struct {
	bq.Table
	dataset string
	project string
	table   string
	meta    *bigquery.TableMetadata
}

func newMockTable(table, dataset, project string, typ bigquery.TableType, lmd time.Time, rows uint64) mockTable {
	return mockTable{
		dataset: dataset,
		project: project,
		table:   table,
		meta:    &bigquery.TableMetadata{Type: typ, LastModifiedTime: lmd, NumRows: rows},
	}
}

func newMockTableDefaults(table string) mockTable {
	return newMockTable(table, "", "", bigquery.RegularTable, time.Now(), 0)
}

func (m mockTable) DatasetID() string {
	return m.dataset
}

func (m mockTable) FullyQualifiedName() string {
	sb := strings.Builder{}
	sb.WriteString("projects/")
	sb.WriteString(m.project)
	sb.WriteString("/datasets/")
	sb.WriteString(m.dataset)
	sb.WriteString("/tables/")
	sb.WriteString(m.table)
	return sb.String()
}

func (m mockTable) Metadata(_ context.Context) (*bigquery.TableMetadata, error) {
	return m.meta, nil
}

func (m mockTable) ProjectID() string {
	return m.project
}

func (m mockTable) TableID() string {
	return m.table
}
