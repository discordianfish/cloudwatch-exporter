package mock

import (
	"context"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

func TestMockCloudwatchEmtpy(t *testing.T) {
	c := NewCloudwatchAPIClient()
	_, err := c.ListMetrics(context.TODO(), &cloudwatch.ListMetricsInput{Namespace: aws.String("AWS/EC2"), MetricName: aws.String("NetworkIn")})
	if err != nil {
		t.Fatal(err)
	}
}
func TestMockCloudwatch(t *testing.T) {
	var (
		metricName       = "NetworkIn"
		namespace        = "AWS/EC2"
		count            = 567
		expectedPageSize = 500
	)
	c := NewCloudwatchAPIClient()
	c.InsertRandom(namespace, metricName, count)

	lmo, err := c.ListMetrics(context.TODO(), &cloudwatch.ListMetricsInput{Namespace: &namespace, MetricName: &metricName})
	if err != nil {
		t.Fatal(err)
	}
	if *lmo.NextToken != strconv.Itoa(expectedPageSize) {
		t.Fatalf("Expected %d but got %s", count-expectedPageSize, *lmo.NextToken)
	}

	queries := make([]types.MetricDataQuery, len(lmo.Metrics))
	for i, m := range lmo.Metrics {
		queries[i] = types.MetricDataQuery{
			MetricStat: &types.MetricStat{
				Metric: &m,
			},
		}
	}
	gmo, err := c.GetMetricData(context.TODO(), &cloudwatch.GetMetricDataInput{
		MetricDataQueries: queries,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gmoc := len(gmo.MetricDataResults); gmoc != expectedPageSize {
		t.Fatalf("Expected %d but got %d", count, gmoc)
	}
}
