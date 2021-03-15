package mock

import (
	"context"
	"reflect"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go/aws"
)

type CloudwatchAPIClient struct {
	cloudwatch.ListMetricsAPIClient
	cloudwatch.GetMetricDataAPIClient

	batchSize int
	metrics   map[string]map[string][]types.Metric
}

func NewCloudwatchAPIClient() *CloudwatchAPIClient {
	return &CloudwatchAPIClient{
		batchSize: 500,
		metrics:   make(map[string]map[string][]types.Metric),
	}
}

func (c *CloudwatchAPIClient) getMetrics(namespace, name *string) []types.Metric {
	metrics := []types.Metric{}
	if namespace == nil {
		for _, nms := range c.metrics {
			for _, ms := range nms {
				metrics = append(metrics, ms...)
			}
		}
		return metrics
	}

	if name == nil {
		for _, ms := range c.metrics[*namespace] {
			metrics = append(metrics, ms...)
		}
		return metrics
	}
	return c.metrics[*namespace][*name]
}

func (c *CloudwatchAPIClient) ListMetrics(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
	start := 0
	if params.NextToken != nil {
		s, err := strconv.Atoi(*params.NextToken)
		if err != nil {
			panic(err)
		}
		start = s
	}
	end := start + c.batchSize

	metrics := c.getMetrics(params.Namespace, params.MetricName)
	l := len(metrics)

	if l < end {
		return &cloudwatch.ListMetricsOutput{
			Metrics: metrics[start:],
		}, nil
	}
	return &cloudwatch.ListMetricsOutput{
		Metrics:   metrics[start:end],
		NextToken: aws.String(strconv.Itoa(end)),
	}, nil
}

func (c *CloudwatchAPIClient) GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	results := &cloudwatch.GetMetricDataOutput{
		MetricDataResults: []types.MetricDataResult{},
	}

	for _, query := range params.MetricDataQueries {
		qmetric := query.MetricStat.Metric
		for _, metric := range c.metrics[*qmetric.Namespace][*qmetric.MetricName] {
			if reflect.DeepEqual(*qmetric, metric) {
				results.MetricDataResults = append(results.MetricDataResults, types.MetricDataResult{
					Id:     query.Id,
					Values: []float64{23.42},
				})
				break
			}
		}
	}
	return results, nil
}

func (c *CloudwatchAPIClient) Insert(namespace, metricName string, dims map[string]string) {
	metric := types.Metric{
		Namespace:  &namespace,
		MetricName: &metricName,
	}
	for k, v := range dims {
		metric.Dimensions = append(metric.Dimensions, types.Dimension{&k, &v})
	}
	if c.metrics[namespace] == nil {
		c.metrics[namespace] = map[string][]types.Metric{}
	}
	c.metrics[namespace][metricName] = append(c.metrics[namespace][metricName], metric)
}

func (c *CloudwatchAPIClient) InsertRandom(namespace, metricName string, count int) {
	for i := 0; i < count; i++ {
		c.Insert(namespace, metricName, map[string]string{"foo": "bar-" + strconv.Itoa(count)})
	}
}
