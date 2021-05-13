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
)

type reporterConfig struct {
	delayDuration time.Duration
	rangeDuration time.Duration
	period        int32
	stat          string
}

type reporter struct {
	config *reporterConfig
	cloudwatch.ListMetricsAPIClient
	cloudwatch.GetMetricDataAPIClient
	logger log.Logger
}

func newReporter(logger log.Logger, rconfig *reporterConfig) (*reporter, error) {
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
	}, nil
}

func (c *reporter) ListMetrics(namespace, metricName string) ([]types.Metric, error) {
	input := &cloudwatch.ListMetricsInput{}
	if metricName != "*" {
		input.MetricName = &metricName
	}
	if namespace != "*" {
		input.Namespace = &namespace
	}

	p := cloudwatch.NewListMetricsPaginator(c.ListMetricsAPIClient, input)
	metrics := []types.Metric{}
	for p.HasMorePages() {
		results, err := p.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}
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

	for i, metric := range metrics {
		metricDataQueries[i] = types.MetricDataQuery{
			Id: aws.String("n" + strconv.Itoa(i)),
			MetricStat: &types.MetricStat{
				Metric: &metric,
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
		r, err := p.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}
		results = append(results, r.MetricDataResults...)
	}
	return results, nil
}
