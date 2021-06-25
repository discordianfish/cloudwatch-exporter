package main

import (
	"context"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

type reporterConfig struct {
	delayDuration time.Duration
	rangeDuration time.Duration
	period        int32
	stat          string
}

type reporter struct {
	config     *reporterConfig
	namespace  string // FIXME: move to config?
	metricName string
	cloudwatch.ListMetricsAPIClient
	cloudwatch.GetMetricDataAPIClient
	logger          log.Logger
	durationSummary *prometheus.SummaryVec
}

func newReporter(logger log.Logger, rconfig *reporterConfig, durationSummary *prometheus.SummaryVec) (*reporter, error) {
	cwc, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := cloudwatch.NewFromConfig(cwc)
	return &reporter{
		config:                 rconfig,
		ListMetricsAPIClient:   client,
		GetMetricDataAPIClient: client,
		logger:                 logger,
		durationSummary:        durationSummary,
	}, nil
}

func (c *reporter) ListMetrics() ([]types.Metric, error) {
	input := &cloudwatch.ListMetricsInput{}
	if c.metricName != "*" {
		input.MetricName = &c.metricName
	}
	if c.namespace != "*" {
		input.Namespace = &c.namespace
	}

	p := cloudwatch.NewListMetricsPaginator(c.ListMetricsAPIClient, input)
	metrics := []types.Metric{}
	for p.HasMorePages() {
		start := time.Now()
		results, err := p.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}
		c.durationSummary.WithLabelValues(c.namespace, c.metricName, "ListMetrics").Observe(time.Since(start).Seconds())
		metrics = append(metrics, results.Metrics...)
	}

	return metrics, nil
}

func (c *reporter) GetMetricsResults(metrics []types.Metric) ([]types.MetricDataResult, error) {
	var (
		now               = time.Now()
		startDate         = now.Add(-(c.config.delayDuration + c.config.rangeDuration))
		endDate           = now.Add(-c.config.delayDuration)
		results           = []types.MetricDataResult{}
		metricDataQueries = make([]types.MetricDataQuery, len(metrics))
	)

	for i := range metrics {
		metricDataQueries[i] = types.MetricDataQuery{
			Id: aws.String("n" + strconv.Itoa(i)),
			MetricStat: &types.MetricStat{
				Metric: &metrics[i],
				Period: &c.config.period,
				Stat:   &c.config.stat,
			},
		}
	}

	p := cloudwatch.NewGetMetricDataPaginator(
		c.GetMetricDataAPIClient,
		&cloudwatch.GetMetricDataInput{
			StartTime:         &startDate,
			EndTime:           &endDate,
			MetricDataQueries: metricDataQueries,
		})

	for p.HasMorePages() {
		start := time.Now()
		r, err := p.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}
		c.durationSummary.WithLabelValues(c.namespace, c.metricName, "GetMetricsResults").Observe(time.Since(start).Seconds())
		results = append(results, r.MetricDataResults...)
	}
	return results, nil
}
