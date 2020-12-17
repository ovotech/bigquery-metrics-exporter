package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"github.com/rs/zerolog/log"
	"net/http"
)

type DatadogPublisher struct {
	cfg *config.Config
}

// NewDatadogPublisher returns a new DatadogPublisher
func NewDatadogPublisher(cfg *config.Config) *DatadogPublisher {
	return &DatadogPublisher{cfg: cfg}
}

// PublishMetricsSet takes a list of metrics and publishes them to Datadog
func (dp *DatadogPublisher) PublishMetricsSet(ctx context.Context, metrics []Metric) error {
	type Request struct {
		Series []Metric `json:"series"`
	}

	log.Info().
		Int("metrics_count", len(metrics)).
		Msg("Publishing metrics to datadog")

	reqBody := Request{Series: metrics}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return NewUnrecoverableError(err)
	}

	url := fmt.Sprintf("https://api.datadoghq.com/api/v1/series?api_key=%s", dp.cfg.DatadogApiKey)
	request, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return NewUnrecoverableError(err)
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return NewRecoverableError(err)
	}

	switch {
	case resp.StatusCode >= 500, resp.StatusCode == 429:
		return NewRecoverableError(err)
	case resp.StatusCode >= 400:
		return NewUnrecoverableError(err)
	}

	return nil
}
