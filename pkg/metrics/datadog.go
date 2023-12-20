package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"net/http"
	"time"
)

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DatadogPublisher publishes slices of Metric to Datadog
type DatadogPublisher struct {
	cfg    *config.Config
	client httpClient
}

// NewDatadogPublisher returns a new DatadogPublisher
func NewDatadogPublisher(cfg *config.Config) *DatadogPublisher {
	client := &http.Client{Timeout: 10 * time.Second}
	return &DatadogPublisher{cfg: cfg, client: client}
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

	ddSite := config.DatadogSites[dp.cfg.DatadogSite]
	url := fmt.Sprintf("https://api.%s/api/v1/series?api_key=%s", ddSite, dp.cfg.DatadogAPIKey)
	request, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return NewUnrecoverableError(err)
	}

	resp, err := dp.client.Do(request)
	if err != nil {
		return NewRecoverableError(err)
	}

	if _, err = ioutil.ReadAll(resp.Body); err != nil {
		return NewRecoverableError(err)
	}

	if err = resp.Body.Close(); err != nil {
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
