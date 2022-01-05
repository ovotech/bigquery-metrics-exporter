package metrics

import (
	"context"
	"fmt"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"github.com/rs/zerolog/log"
	"sort"
	"strings"
	"sync"
	"time"
)

// Metric represents a metric to submit and a list of readings of that metric
type Metric struct {
	Interval uint64      `json:"interval"`
	Metric   string      `json:"metric"`
	Points   [][]float64 `json:"points"`
	Tags     []string    `json:"tags"`
	Type     string      `json:"type"`
}

// TypeGauge is a "gauge" type metric
const TypeGauge = "gauge"

// ID returns an identifier for the metric
func (m *Metric) ID() string {
	sb := strings.Builder{}
	sb.WriteString(m.Metric)
	for _, tag := range m.Tags {
		sb.WriteRune(';')
		sb.WriteString(tag)
	}
	return sb.String()
}

// Reading is a point in time reading of some metric
type Reading struct {
	Timestamp time.Time
	Value     float64
}

// NewReading creates a new reading with the current timestamp
func NewReading(val float64) Reading {
	return Reading{
		Timestamp: time.Now(),
		Value:     val,
	}
}

// NewReadingFrom creates a new reading from an interface type if possible
func NewReadingFrom(val interface{}, at time.Time) (Reading, error) {
	switch val := val.(type) {
	case bool:
		if val {
			return Reading{Timestamp: at, Value: 1.0}, nil
		}
		return Reading{Timestamp: at, Value: 0.0}, nil
	case time.Time:
		return Reading{Timestamp: at, Value: float64(val.Unix())}, nil
	case int:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case int8:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case int16:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case int32:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case int64:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case uint:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case uint8:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case uint16:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case uint32:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case uint64:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case float32:
		return Reading{Timestamp: at, Value: float64(val)}, nil
	case float64:
		return Reading{Timestamp: at, Value: val}, nil
	}

	return Reading{}, ErrInvalidReadingType
}

func (r Reading) serialize() []float64 {
	return []float64{float64(r.Timestamp.Unix()), r.Value}
}

func (m *Metric) append(r Reading) {
	pm := createPointmap(m)

	point := r.serialize()
	if _, ok := pm[point[0]]; ok {
		*pm[point[0]] = point[1]
	} else {
		m.Points = append(m.Points, point)
	}
}

func (m *Metric) mergePoints(o *Metric) {
	pm := createPointmap(m)

	for _, point := range o.Points {
		if len(point) != 2 {
			continue
		}

		if _, ok := pm[point[0]]; ok {
			*pm[point[0]] = point[1]
		} else {
			m.Points = append(m.Points, point)
		}
	}
}

// Producer can create new metrics
type Producer struct {
	config *config.Config
}

// NewProducer returns a metric Producer
func NewProducer(c *config.Config) Producer {
	return Producer{config: c}
}

// Produce creates a metric from a given Reading, based on current configuration
func (p *Producer) Produce(metric string, read Reading, tags []string) *Metric {
	tags = append(tags, p.config.MetricTags...)
	sort.Strings(tags)

	return &Metric{
		Interval: uint64(p.config.MetricInterval.Seconds()),
		Metric:   getFullMetricName(p.config.MetricPrefix, metric),
		Points:   [][]float64{read.serialize()},
		Tags:     tags,
		Type:     TypeGauge,
	}
}

func getFullMetricName(prefix, metric string) string {
	sb := strings.Builder{}
	sb.WriteString(prefix)
	if prefix != "" && !strings.HasSuffix(prefix, ".") {
		sb.WriteRune('.')
	}
	if strings.HasPrefix(metric, ".") {
		metric = strings.TrimPrefix(metric, ".")
	}
	sb.WriteString(metric)
	return sb.String()
}

type pointmap map[float64]*float64

func createPointmap(m *Metric) pointmap {
	pm := make(pointmap)
	for _, point := range m.Points {
		if len(point) != 2 {
			continue
		}

		pm[point[0]] = &point[1]
	}
	return pm
}

// Consumer consumes metrics, storing them in an internal map and maintaining
// a consistent view of currently unpublished metrics
type Consumer struct {
	mx      sync.Mutex
	metrics map[string]*Metric
}

// NewConsumer is a factory for creating a Consumer
func NewConsumer() *Consumer {
	var metrics map[string]*Metric
	metrics = make(map[string]*Metric)

	return &Consumer{metrics: metrics}
}

// Run will run the consumer, returning a channel to feed metrics into
func (c *Consumer) Run(ctx context.Context, wg *sync.WaitGroup) chan *Metric {
	log.Debug().Msg("Starting metric consumer")

	receiver := make(chan *Metric)

	go func() {
		defer wg.Done()

		for {
			select {
			case metric := <-receiver:
				c.consume(metric)
			case <-ctx.Done():
				close(receiver)
				return
			}
		}
	}()
	return receiver
}

// Flush will return all currently consumed metrics and empty the metric buffer
func (c *Consumer) Flush() []Metric {
	c.mx.Lock()
	defer c.mx.Unlock()

	metrics := c.getMetrics()
	c.metrics = make(map[string]*Metric)
	return metrics
}

type publisher interface {
	PublishMetricsSet(context.Context, []Metric) error
}

// PublishTo will publish the metrics collected so far to the provided publisher
// If a recoverable error is encountered, the metric buffer is not flushed so that
// the metrics can be resent during the next publishing attempt
func (c *Consumer) PublishTo(ctx context.Context, pub publisher) error {
	c.mx.Lock()
	defer c.mx.Unlock()

	metrics := c.getMetrics()
	if len(metrics) == 0 {
		log.Debug().
			Int("metrics_count", 0).
			Msg("No metrics to publish")

		return nil
	}

	err := pub.PublishMetricsSet(ctx, metrics)
	if IsRecoverable(err) {
		return fmt.Errorf("error publishing %d metrics, %w", len(metrics), err)
	}

	c.metrics = make(map[string]*Metric)
	return err
}

func (c *Consumer) getMetrics() []Metric {
	var metrics []Metric
	metrics = make([]Metric, len(c.metrics))

	i := 0
	for _, metric := range c.metrics {
		metrics[i] = *metric
		i++
	}

	return metrics
}

func (c *Consumer) consume(m *Metric) {
	c.mx.Lock()
	defer c.mx.Unlock()

	if _, ok := c.metrics[m.ID()]; ok {
		c.metrics[m.ID()].mergePoints(m)
	} else {
		c.metrics[m.ID()] = m
	}
}
