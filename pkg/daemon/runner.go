package daemon

import (
	"context"
	"fmt"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/metrics"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/sources"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
)

// Generator defines something that is able to output *metrics.Metric into a channel
type Generator interface {
	ProduceMetrics(context.Context, chan *metrics.Metric)
	ProduceCustomMetric(context.Context, config.CustomMetric, chan *metrics.Metric)
}

// Publisher defines something that is able to publish a slice of metrics.Metric
type Publisher interface {
	PublishMetricsSet(context.Context, []metrics.Metric) error
}

// Runner co-ordinates metric generation and metric publishing
type Runner struct {
	cfg       *config.Config
	consumer  *metrics.Consumer
	generator Generator
	publisher Publisher
}

// NewRunner returns a Runner instance configured appropriately
func NewRunner(ctx context.Context, cfg *config.Config) (*Runner, error) {
	generator, err := sources.NewGenerator(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating metrics Generator: %w", err)
	}

	return &Runner{
		cfg:       cfg,
		consumer:  metrics.NewConsumer(),
		generator: generator,
		publisher: metrics.NewDatadogPublisher(cfg),
	}, nil
}

// RunOnce runs a single round of metrics collection and submits them
// to DataDog immediately
func (d *Runner) RunOnce(ctx context.Context) error {
	log.Info().Msg("Starting Runner")

	receiver := d.consumer.Run()
	defer close(receiver)

	wg := sync.WaitGroup{}
	wg.Add(len(d.cfg.CustomMetrics))
	for _, m := range d.cfg.CustomMetrics {
		go func(cm config.CustomMetric) {
			d.generator.ProduceCustomMetric(ctx, cm, receiver)
			wg.Done()
		}(m)
	}
	wg.Wait()

	d.generator.ProduceMetrics(ctx, receiver)
	err := d.consumer.PublishTo(ctx, d.publisher)

	log.Err(err).Msg("Finishing Runner")

	return err
}

// RunUntil runs the metrics collection process in one goroutine and
// the submission process in another goroutine, and runs them until the
// context is cancelled
func (d *Runner) RunUntil(ctx context.Context) error {
	log.Info().Msg("Starting Runner")

	var abort context.CancelFunc
	ctx, abort = context.WithCancel(ctx)

	receiver := d.consumer.Run()
	defer close(receiver)

	var problem chan error
	problem = make(chan error, 1)

	wg := sync.WaitGroup{}
	wg.Add(2 + len(d.cfg.CustomMetrics))

	go d.startMetricPublisher(ctx, abort, &wg, problem)
	go d.startTableMetricsGenerator(ctx, &wg, receiver)
	for _, cm := range d.cfg.CustomMetrics {
		go d.startCustomMetricsGenerator(ctx, cm, &wg, receiver)
	}

	wg.Wait()

	select {
	case err := <-problem:
		log.Err(err).Msg("Finishing Runner")

		return err
	default:
		log.Info().Msg("Finishing Runner")

		return nil
	}
}

func (d *Runner) startMetricPublisher(ctx context.Context, abort context.CancelFunc, wg *sync.WaitGroup, problem chan error) {
	logger := log.With().
		Str("component", "Publisher").
		Str("metric_interval", d.cfg.MetricInterval.String()).
		Str("metric_prefix", d.cfg.MetricPrefix).
		Logger()
	logger.Info().Msg("Starting metric publishing")

	ticker := time.NewTicker(d.cfg.MetricInterval)
	defer ticker.Stop()
	defer wg.Done()

	for {
		select {
		case <-ticker.C:
			err := d.consumer.PublishTo(ctx, d.publisher)
			if metrics.IsUnrecoverable(err) {
				logger.Err(err).
					Msg("Unrecoverable error occurred when publishing, finishing metric production goroutine. Metric data will be lost")

				problem <- err
				abort()
				return
			}
		case <-ctx.Done():
			logger.Info().Msg("Received end signal, performing final metric publishing")

			finalCtx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
			err := d.consumer.PublishTo(finalCtx, d.publisher)
			cancel()
			if err != nil {
				logger.Err(err).
					Msg("Error during final metric publishing. Metric data will be lost")

				problem <- err
			}
			return
		}
	}
}

func (d *Runner) startCustomMetricsGenerator(ctx context.Context, cm config.CustomMetric, wg *sync.WaitGroup, receiver chan *metrics.Metric) {
	logger := log.With().
		Str("component", "Custom Generator").
		Str("metric_interval", cm.MetricInterval.String()).
		Str("metric_name", cm.MetricName).
		Str("metric_prefix", d.cfg.MetricPrefix).
		Logger()
	logger.Info().Msg("Starting custom metric production")

	ticker := time.NewTicker(cm.MetricInterval)
	defer ticker.Stop()
	defer wg.Done()

	for {
		select {
		case <-ticker.C:
			d.generator.ProduceCustomMetric(ctx, cm, receiver)
		case <-ctx.Done():
			logger.Info().Msg("Received end signal, finishing metric production")
			return
		}
	}
}

func (d *Runner) startTableMetricsGenerator(ctx context.Context, wg *sync.WaitGroup, receiver chan *metrics.Metric) {
	logger := log.With().
		Str("component", "Generator").
		Str("metric_interval", d.cfg.MetricInterval.String()).
		Str("metric_prefix", d.cfg.MetricPrefix).
		Logger()
	logger.Info().Msg("Starting table metric production")

	ticker := time.NewTicker(d.cfg.MetricInterval)
	defer ticker.Stop()
	defer wg.Done()

	for {
		select {
		case <-ticker.C:
			d.generator.ProduceMetrics(ctx, receiver)
		case <-ctx.Done():
			logger.Info().Msg("Received end signal, finishing metric production")
			return
		}
	}
}
