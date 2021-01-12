package sources

import (
	"cloud.google.com/go/bigquery"
	"context"
	"fmt"
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
	client   *bigquery.Client
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
		client:   client,
		producer: metrics.NewProducer(cfg),
	}, nil
}

// ProduceMetrics will generate table level metrics for all BigQuery tables
func (g Generator) ProduceMetrics(ctx context.Context, receiver chan *metrics.Metric) {
	log.Debug().Msg("Producing table level metrics")

	wg := sync.WaitGroup{}
	for ds := range iterateDatasets(ctx, g.client) {
		for tbl := range iterateTables(ctx, ds) {
			wg.Add(1)
			go g.outputTableLevelMetrics(ctx, tbl, receiver, &wg)
		}
	}
	wg.Wait()
}

func (g Generator) outputTableLevelMetrics(ctx context.Context, t *bigquery.Table, out chan *metrics.Metric, wg *sync.WaitGroup) {
	defer wg.Done()

	meta, err := t.Metadata(ctx)
	if err != nil {
		log.Err(err).
			Str("project_id", t.ProjectID).
			Str("dataset_id", t.DatasetID).
			Str("table_id", t.TableID).
			Msg("An error occurred when fetching table metadata")

		return
	}

	// Only tables of type RegularTable and MaterializedView have the required metadata present
	if meta.Type == bigquery.ViewTable || meta.Type == bigquery.ExternalTable {
		return
	}

	tags := []string{
		fmt.Sprintf("dataset_id:%s", t.DatasetID),
		fmt.Sprintf("table_id:%s", t.TableID),
		fmt.Sprintf("project_id:%s", t.ProjectID),
	}
	now := time.Now().Unix()
	out <- g.producer.Produce("row_count", metrics.NewReading(float64(meta.NumRows)), tags)
	out <- g.producer.Produce("last_modified_time", metrics.NewReading(float64(meta.LastModifiedTime.Unix())), tags)
	out <- g.producer.Produce("last_modified", metrics.NewReading(float64(now)-float64(meta.LastModifiedTime.Unix())), tags)
}

func iterateDatasets(ctx context.Context, client *bigquery.Client) chan *bigquery.Dataset {
	var out chan *bigquery.Dataset
	out = make(chan *bigquery.Dataset)

	go func() {
		defer close(out)
		iter := client.Datasets(ctx)

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

func iterateTables(ctx context.Context, ds *bigquery.Dataset) chan *bigquery.Table {
	var out chan *bigquery.Table
	out = make(chan *bigquery.Table)

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
					Str("project_id", ds.ProjectID).
					Str("dataset_id", ds.DatasetID).
					Msg("An error occurred when fetching table information")

				break
			}

			out <- tbl
		}
	}()

	return out
}
