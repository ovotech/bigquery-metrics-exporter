package sources

import (
	"cloud.google.com/go/bigquery"
	"context"
	"fmt"
	bq "github.com/googleapis/google-cloud-go-testing/bigquery/bqiface"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/metrics"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
	"sync"
	"time"
)

// Generator can generate metrics from BigQuery tables
type Generator struct {
	cfg      *config.Config
	client   bq.Client
	producer metrics.Producer
}

// NewGenerator returns a new BigQuery metrics Generator
func NewGenerator(ctx context.Context, cfg *config.Config) (*Generator, error) {
	client, err := bigquery.NewClient(ctx, cfg.GcpProject)
	if err != nil {
		return nil, fmt.Errorf("error creating BigQuery client: %w", err)
	}

	return &Generator{
		cfg:      cfg,
		client:   bq.AdaptClient(client),
		producer: metrics.NewProducer(cfg),
	}, nil
}

// ProduceMetrics will generate table level metrics for all BigQuery tables
func (g Generator) ProduceMetrics(ctx context.Context, receiver chan *metrics.Metric) {
	log.Debug().Str("dataset-filter", g.cfg.DatasetFilter).Msg("Producing table level metrics")

	wg := sync.WaitGroup{}
	for ds := range iterateDatasets(ctx, g.client, g.cfg.DatasetFilter) {
		for tbl := range iterateTables(ctx, ds) {
			wg.Add(1)
			go g.outputTableLevelMetrics(ctx, tbl, receiver, &wg)
		}
	}
	wg.Wait()
}

// ProduceCustomMetric will generate a metric based on a CustomMetric
func (g Generator) ProduceCustomMetric(ctx context.Context, cm config.CustomMetric, out chan *metrics.Metric) {
	logger := log.With().
		Str("metric-name", cm.MetricName).
		Str("sql", cm.SQL).
		Logger()

	logger.Debug().Msg("Producing custom metric")

	iter, err := g.runSQLQuery(ctx, cm.SQL)
	if err != nil {
		logger.Err(err).Msg("Error occurred reading custom query")
		return
	}

	now := time.Now()

	var results map[string]bigquery.Value
	err = iter.Next(&results)
	if err != nil {
		if err == iterator.Done {
			logger.Info().Msg("Query returned no results")
			return
		}

		logger.Err(err).Msg("Query results iterator produced an error")
		return
	}

	if iter.TotalRows() > 1 {
		logger.Warn().Msg("Query returned multiple rows but only the first row is used")
	}

	for colName, colVal := range results {
		reading, err := metrics.NewReadingFrom(colVal, now)
		if err != nil {
			logger.Err(err).
				Str("column_id", colName).
				Msg("Query results must be of numeric type")
			continue
		}
		tags := append(
			[]string{fmt.Sprintf("column_id:%s", colName)},
			cm.MetricTags...,
		)

		out <- g.producer.Produce(fmt.Sprintf("custom_metric.%s", cm.MetricName), reading, tags)
	}
}

func (g Generator) runSQLQuery(ctx context.Context, sql string) (bq.RowIterator, error) {
	q := g.client.Query(sql)

	cfg := bq.QueryConfig{}
	cfg.Q = sql
	cfg.Labels = make(map[string]string)
	cfg.Labels["created-by"] = config.AppName

	q.SetQueryConfig(cfg)

	job, err := q.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("error running query: %w", err)
	}

	iter, err := job.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading query results: %w", err)
	}

	return iter, nil
}

func (g Generator) outputTableLevelMetrics(ctx context.Context, t bq.Table, out chan *metrics.Metric, wg *sync.WaitGroup) {
	defer wg.Done()

	meta, err := t.Metadata(ctx)
	if err != nil {
		log.Err(err).
			Str("project_id", t.ProjectID()).
			Str("dataset_id", t.DatasetID()).
			Str("table_id", t.TableID()).
			Msg("An error occurred when fetching table metadata")

		return
	}

	// Only tables of type RegularTable and MaterializedView have the required metadata present
	if meta.Type == bigquery.ViewTable || meta.Type == bigquery.ExternalTable {
		return
	}

	tags := []string{
		fmt.Sprintf("dataset_id:%s", t.DatasetID()),
		fmt.Sprintf("table_id:%s", t.TableID()),
		fmt.Sprintf("project_id:%s", t.ProjectID()),
	}
	now := time.Now().Unix()
	out <- g.producer.Produce("table.row_count", metrics.NewReading(float64(meta.NumRows)), tags)
	out <- g.producer.Produce("table.last_modified_time", metrics.NewReading(float64(meta.LastModifiedTime.Unix())), tags)
	out <- g.producer.Produce("table.last_modified", metrics.NewReading(float64(now)-float64(meta.LastModifiedTime.Unix())), tags)
}

func iterateDatasets(ctx context.Context, client bq.Client, filter string) chan bq.Dataset {
	var out chan bq.Dataset
	out = make(chan bq.Dataset)

	go func() {
		defer close(out)
		iter := client.Datasets(ctx)
		iter.SetFilter(filter)

		for {
			ds, err := iter.Next()
			if err != nil {
				if err == iterator.Done {
					break
				}

				log.Err(err).
					Msg("An error occurred when fetching dataset information")

				break
			}

			out <- ds
		}
	}()

	return out
}

func iterateTables(ctx context.Context, ds bq.Dataset) chan bq.Table {
	var out chan bq.Table
	out = make(chan bq.Table)

	go func() {
		defer close(out)
		iter := ds.Tables(ctx)

		for {
			tbl, err := iter.Next()
			if err != nil {
				if err == iterator.Done {
					break
				}

				log.Err(err).
					Str("project_id", ds.ProjectID()).
					Str("dataset_id", ds.DatasetID()).
					Msg("An error occurred when fetching table information")

				break
			}

			out <- tbl
		}
	}()

	return out
}
