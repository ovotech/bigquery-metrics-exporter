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

	receiver := d.consumer.Run()
	defer close(receiver)

	var abort chan struct{}
	abort = make(chan struct{})

	var problem chan error
	problem = make(chan error, 1)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		logger := log.With().
			Str("component", "Generator").
			Str("metric_interval", d.cfg.MetricInterval.String()).
			Str("metric_prefix", d.cfg.MetricPrefix).
			Logger()
		logger.Info().Msg("Starting metric production")

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
			case <-abort:
				logger.Info().Msg("Received abort signal, finishing metric production")
				return
			}
		}
	}()
	go func() {
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
					close(abort)
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
	}()

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
