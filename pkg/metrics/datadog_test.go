package metrics

import (
	"bytes"
	"context"
	"github.com/ovotech/bigquery-metrics-extractor/pkg/config"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

type mockHttpClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHttpClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestDatadogPublisher_PublishMetricsSet(t *testing.T) {
	apiKey := "ABC123"
	metrics := []Metric{
		{
			Interval: 60,
			Metric: "value",
			Points: [][]float64{{1.0, 1.0}, {2.0, 1.0}},
			Tags: []string{"env:nonprod"},
			Type: TypeGauge,
		},
	}

	testRequest := func(test func(t *testing.T, req *http.Request)) func(*testing.T) func(*http.Request) (*http.Response, error) {
		return func(t *testing.T) func(req *http.Request) (*http.Response, error) {
			return func(req *http.Request) (*http.Response, error) {
				test(t, req)

				return &http.Response{
					Body:       ioutil.NopCloser(bytes.NewReader([]byte(""))),
					StatusCode: 200,
				}, nil
			}
		}
	}

	testResponse := func(body string, status int) func(*testing.T) func(*http.Request) (*http.Response, error) {
		return func(_ *testing.T) func(req *http.Request) (*http.Response, error) {
			return func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					Body:       ioutil.NopCloser(bytes.NewReader([]byte(body))),
					StatusCode: status,
				}, nil
			}
		}
	}

	type args struct {
		metrics []Metric
	}
	tests := []struct {
		name    string
		dofunc  func(t *testing.T) func(req *http.Request) (*http.Response, error)
		args    args
		wantErr error
	}{
		{
			"API Key sent on request",
			testRequest(func(t *testing.T, req *http.Request) {
				if ! strings.Contains(req.URL.String(), apiKey) {
					t.Errorf("Request URL %s did not contain API key %s", req.URL.String(), apiKey)
				}
			}),
			args{metrics: metrics},
			nil,
		},
		{
			"Metrics sent on request",
			testRequest(func(t *testing.T, req *http.Request) {
				body, _ := ioutil.ReadAll(req.Body)
				expected := "{\"series\":[{\"interval\":60,\"metric\":\"value\",\"points\":[[1,1],[2,1]],\"tags\":[\"env:nonprod\"],\"type\":\"gauge\"}]}"
				if string(body) != expected {
					t.Errorf("Request body %s did not match expected body %s", string(body), expected)
				}
			}),
			args{metrics: metrics},
			nil,
		},
		{
			"Datadog internal error",
			testResponse("{\"errors\": [\"Internal Server Error\"]}", 500),
			args{metrics: metrics},
			NewRecoverableError(nil),
		},
		{
			"Datadog bad request error",
			testResponse("{\"errors\": [\"Bad Request\"]}", 400),
			args{metrics: metrics},
			NewUnrecoverableError(nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dp := &DatadogPublisher{
				cfg:    &config.Config{DatadogAPIKey: apiKey},
				client: &mockHttpClient{tt.dofunc(t)},
			}
			err := dp.PublishMetricsSet(context.TODO(), tt.args.metrics)
			if err != tt.wantErr {
				t.Errorf("PublishMetricsSet() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
