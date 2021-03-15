package main

import (
	"testing"

	"github.com/discordianfish/cloudwatch-exporter/mock"
)

func TestReporter(t *testing.T) {
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

	reporter := &reporter{
		ListMetricsAPIClient:   client,
		GetMetricDataAPIClient: client,
	}

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
		metrics, err := reporter.ListMetrics(tc.namespace, tc.metricName)
		if err != nil {
			t.Fatal(err)
		}
		if c := len(metrics); c != tc.count {
			t.Fatalf("Expected %d but got %d results", tc.count, c)
		}
	}
}
