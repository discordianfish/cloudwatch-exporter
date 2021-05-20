package main

import (
	"os"
	"testing"
	"time"

	"github.com/discordianfish/cloudwatch-exporter/mock"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

func TestCollector(t *testing.T) {
	var (
		metricNames = []string{"NetworkIn", "NetworkOut", "NetworkPacketsIn", "NetworkPacketsOut"}
		count       = 567 // >500 to force pagination
	)
	client := mock.NewCloudwatchAPIClient()
	// populate metrics
	for _, mn := range metricNames {
		client.InsertRandom("AWS/EC2", mn, count)
	}
	client.InsertRandom("AWS/EBS", "VolumeWriteBytes", count)

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	for _, tc := range []struct {
		namespace  string
		metricName string
		count      int
	}{
		{"AWS/EC2", "NetworkIn", count},
		{"AWS/EC2", "*", count * len(metricNames)},
		{"AWS/EBS", "VolumeWriteBytes", count},
		{"AWS/EBS", "*", count},
		{"*", "*", count * (len(metricNames) + 1)}, // Also returns the EBS metric
	} {
		reporter := &reporter{
			ListMetricsAPIClient:   client,
			GetMetricDataAPIClient: client,
			config: &reporterConfig{
				delayDuration: 600 * time.Second,
				rangeDuration: 600 * time.Second,
				period:        60,
				stat:          "Maximum",
			},
			namespace:  tc.namespace,
			metricName: tc.metricName,
			durationSummary: prometheus.NewSummaryVec(prometheus.SummaryOpts{
				Name: "cloudwatch_request_duration_seconds",
				Help: "Duration of cloudwatch metric collection.",
			}, []string{"metric_namespace", "metric_name", "api_call"}),
		}
		collector := newCollector(logger, reporter,
			prometheus.NewCounter(prometheus.CounterOpts{
				Name: "cloudwatch_errors_total",
				Help: "Number of errors.",
			}),
		)

		metrics := []prometheus.Metric{}

		ch := make(chan prometheus.Metric)
		go func() {
			collector.Collect(ch)
			close(ch)
		}()
		for m := range ch {
			metrics = append(metrics, m)
			t.Logf("Got metric %v", m)
		}

		if c := len(metrics); c != tc.count {
			t.Fatalf("Expected %d but got %d results", tc.count, c)
		}
	}
}
