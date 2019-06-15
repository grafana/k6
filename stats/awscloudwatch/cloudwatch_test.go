package awscloudwatch

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCloudWatchClientToMetricDatumLimitsDimensionsToTen(t *testing.T) {
	s := &sample{
		Tags: map[string]string{
			"proto":       "http",
			"subproto":    "http2",
			"status":      "200",
			"method":      "GET",
			"url":         "https://github.com/loadimpact/k6",
			"name":        "https://github.com/loadimpact/k6",
			"group":       "",
			"check":       "must be 200",
			"error":       "Some error",
			"error_code":  "abcd3",
			"tls_version": "1.2",
			"foo":         "bar",
		},
	}

	require.Equal(
		t, 10, len(toMetricDatum(s).Dimensions),
	)
}

func TestCloudWatchClientReportSamplesPutsDataOnCloudWatch(t *testing.T) {
	fakeAWSCloudWatchClient := newFakeAWSCloudwatchClient()
	c := client{
		CloudWatchAPI: fakeAWSCloudWatchClient,
		namespace:     "some/namespace",
	}

	samples := []*sample{
		{
			Metric: "metric_name",
			Value:  1.0,
			Time:   time.Now(),
			Tags:   map[string]string{"tag": "value"},
		},
	}

	err := c.reportSamples(samples)
	assert.NoError(t, err)

	expectedMetricsData := []*cloudwatch.PutMetricDataInput{{
		Namespace: &c.namespace,
		MetricData: []*cloudwatch.MetricDatum{
			{
				Value:      &samples[0].Value,
				MetricName: &samples[0].Metric,
				Timestamp:  &samples[0].Time,
				Dimensions: []*cloudwatch.Dimension{
					{
						Name:  aws.String("tag"),
						Value: aws.String("value"),
					},
				},
			},
		},
	}}

	assert.Equal(t, fakeAWSCloudWatchClient.metrics, expectedMetricsData)
}

func newFakeAWSCloudwatchClient() *fakeAWSCloudWatchClient {
	return &fakeAWSCloudWatchClient{}
}

type fakeAWSCloudWatchClient struct {
	metrics []*cloudwatch.PutMetricDataInput
}

func (f *fakeAWSCloudWatchClient) PutMetricData(in *cloudwatch.PutMetricDataInput) (*cloudwatch.PutMetricDataOutput, error) {
	f.metrics = append(f.metrics, in)

	return &cloudwatch.PutMetricDataOutput{}, nil
}

// Empty implementations as these methods are not used

func (f *fakeAWSCloudWatchClient) DeleteAlarms(*cloudwatch.DeleteAlarmsInput) (*cloudwatch.DeleteAlarmsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DeleteAlarmsWithContext(aws.Context, *cloudwatch.DeleteAlarmsInput, ...request.Option) (*cloudwatch.DeleteAlarmsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DeleteAlarmsRequest(*cloudwatch.DeleteAlarmsInput) (*request.Request, *cloudwatch.DeleteAlarmsOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DeleteDashboards(*cloudwatch.DeleteDashboardsInput) (*cloudwatch.DeleteDashboardsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DeleteDashboardsWithContext(aws.Context, *cloudwatch.DeleteDashboardsInput, ...request.Option) (*cloudwatch.DeleteDashboardsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DeleteDashboardsRequest(*cloudwatch.DeleteDashboardsInput) (*request.Request, *cloudwatch.DeleteDashboardsOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmHistory(*cloudwatch.DescribeAlarmHistoryInput) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmHistoryWithContext(aws.Context, *cloudwatch.DescribeAlarmHistoryInput, ...request.Option) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmHistoryRequest(*cloudwatch.DescribeAlarmHistoryInput) (*request.Request, *cloudwatch.DescribeAlarmHistoryOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmHistoryPages(*cloudwatch.DescribeAlarmHistoryInput, func(*cloudwatch.DescribeAlarmHistoryOutput, bool) bool) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmHistoryPagesWithContext(aws.Context, *cloudwatch.DescribeAlarmHistoryInput, func(*cloudwatch.DescribeAlarmHistoryOutput, bool) bool, ...request.Option) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarms(*cloudwatch.DescribeAlarmsInput) (*cloudwatch.DescribeAlarmsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmsWithContext(aws.Context, *cloudwatch.DescribeAlarmsInput, ...request.Option) (*cloudwatch.DescribeAlarmsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmsRequest(*cloudwatch.DescribeAlarmsInput) (*request.Request, *cloudwatch.DescribeAlarmsOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmsPages(*cloudwatch.DescribeAlarmsInput, func(*cloudwatch.DescribeAlarmsOutput, bool) bool) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmsPagesWithContext(aws.Context, *cloudwatch.DescribeAlarmsInput, func(*cloudwatch.DescribeAlarmsOutput, bool) bool, ...request.Option) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmsForMetric(*cloudwatch.DescribeAlarmsForMetricInput) (*cloudwatch.DescribeAlarmsForMetricOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmsForMetricWithContext(aws.Context, *cloudwatch.DescribeAlarmsForMetricInput, ...request.Option) (*cloudwatch.DescribeAlarmsForMetricOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DescribeAlarmsForMetricRequest(*cloudwatch.DescribeAlarmsForMetricInput) (*request.Request, *cloudwatch.DescribeAlarmsForMetricOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DisableAlarmActions(*cloudwatch.DisableAlarmActionsInput) (*cloudwatch.DisableAlarmActionsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DisableAlarmActionsWithContext(aws.Context, *cloudwatch.DisableAlarmActionsInput, ...request.Option) (*cloudwatch.DisableAlarmActionsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) DisableAlarmActionsRequest(*cloudwatch.DisableAlarmActionsInput) (*request.Request, *cloudwatch.DisableAlarmActionsOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) EnableAlarmActions(*cloudwatch.EnableAlarmActionsInput) (*cloudwatch.EnableAlarmActionsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) EnableAlarmActionsWithContext(aws.Context, *cloudwatch.EnableAlarmActionsInput, ...request.Option) (*cloudwatch.EnableAlarmActionsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) EnableAlarmActionsRequest(*cloudwatch.EnableAlarmActionsInput) (*request.Request, *cloudwatch.EnableAlarmActionsOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetDashboard(*cloudwatch.GetDashboardInput) (*cloudwatch.GetDashboardOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetDashboardWithContext(aws.Context, *cloudwatch.GetDashboardInput, ...request.Option) (*cloudwatch.GetDashboardOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetDashboardRequest(*cloudwatch.GetDashboardInput) (*request.Request, *cloudwatch.GetDashboardOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricData(*cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricDataWithContext(aws.Context, *cloudwatch.GetMetricDataInput, ...request.Option) (*cloudwatch.GetMetricDataOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricDataRequest(*cloudwatch.GetMetricDataInput) (*request.Request, *cloudwatch.GetMetricDataOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricDataPages(*cloudwatch.GetMetricDataInput, func(*cloudwatch.GetMetricDataOutput, bool) bool) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricDataPagesWithContext(aws.Context, *cloudwatch.GetMetricDataInput, func(*cloudwatch.GetMetricDataOutput, bool) bool, ...request.Option) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricStatistics(*cloudwatch.GetMetricStatisticsInput) (*cloudwatch.GetMetricStatisticsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricStatisticsWithContext(aws.Context, *cloudwatch.GetMetricStatisticsInput, ...request.Option) (*cloudwatch.GetMetricStatisticsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricStatisticsRequest(*cloudwatch.GetMetricStatisticsInput) (*request.Request, *cloudwatch.GetMetricStatisticsOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricWidgetImage(*cloudwatch.GetMetricWidgetImageInput) (*cloudwatch.GetMetricWidgetImageOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricWidgetImageWithContext(aws.Context, *cloudwatch.GetMetricWidgetImageInput, ...request.Option) (*cloudwatch.GetMetricWidgetImageOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) GetMetricWidgetImageRequest(*cloudwatch.GetMetricWidgetImageInput) (*request.Request, *cloudwatch.GetMetricWidgetImageOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListDashboards(*cloudwatch.ListDashboardsInput) (*cloudwatch.ListDashboardsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListDashboardsWithContext(aws.Context, *cloudwatch.ListDashboardsInput, ...request.Option) (*cloudwatch.ListDashboardsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListDashboardsRequest(*cloudwatch.ListDashboardsInput) (*request.Request, *cloudwatch.ListDashboardsOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListDashboardsPages(*cloudwatch.ListDashboardsInput, func(*cloudwatch.ListDashboardsOutput, bool) bool) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListDashboardsPagesWithContext(aws.Context, *cloudwatch.ListDashboardsInput, func(*cloudwatch.ListDashboardsOutput, bool) bool, ...request.Option) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListMetrics(*cloudwatch.ListMetricsInput) (*cloudwatch.ListMetricsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListMetricsWithContext(aws.Context, *cloudwatch.ListMetricsInput, ...request.Option) (*cloudwatch.ListMetricsOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListMetricsRequest(*cloudwatch.ListMetricsInput) (*request.Request, *cloudwatch.ListMetricsOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListMetricsPages(*cloudwatch.ListMetricsInput, func(*cloudwatch.ListMetricsOutput, bool) bool) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListMetricsPagesWithContext(aws.Context, *cloudwatch.ListMetricsInput, func(*cloudwatch.ListMetricsOutput, bool) bool, ...request.Option) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListTagsForResource(*cloudwatch.ListTagsForResourceInput) (*cloudwatch.ListTagsForResourceOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListTagsForResourceWithContext(aws.Context, *cloudwatch.ListTagsForResourceInput, ...request.Option) (*cloudwatch.ListTagsForResourceOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) ListTagsForResourceRequest(*cloudwatch.ListTagsForResourceInput) (*request.Request, *cloudwatch.ListTagsForResourceOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) PutDashboard(*cloudwatch.PutDashboardInput) (*cloudwatch.PutDashboardOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) PutDashboardWithContext(aws.Context, *cloudwatch.PutDashboardInput, ...request.Option) (*cloudwatch.PutDashboardOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) PutDashboardRequest(*cloudwatch.PutDashboardInput) (*request.Request, *cloudwatch.PutDashboardOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) PutMetricAlarm(*cloudwatch.PutMetricAlarmInput) (*cloudwatch.PutMetricAlarmOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) PutMetricAlarmWithContext(aws.Context, *cloudwatch.PutMetricAlarmInput, ...request.Option) (*cloudwatch.PutMetricAlarmOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) PutMetricAlarmRequest(*cloudwatch.PutMetricAlarmInput) (*request.Request, *cloudwatch.PutMetricAlarmOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) PutMetricDataWithContext(aws.Context, *cloudwatch.PutMetricDataInput, ...request.Option) (*cloudwatch.PutMetricDataOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) PutMetricDataRequest(*cloudwatch.PutMetricDataInput) (*request.Request, *cloudwatch.PutMetricDataOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) SetAlarmState(*cloudwatch.SetAlarmStateInput) (*cloudwatch.SetAlarmStateOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) SetAlarmStateWithContext(aws.Context, *cloudwatch.SetAlarmStateInput, ...request.Option) (*cloudwatch.SetAlarmStateOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) SetAlarmStateRequest(*cloudwatch.SetAlarmStateInput) (*request.Request, *cloudwatch.SetAlarmStateOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) TagResource(*cloudwatch.TagResourceInput) (*cloudwatch.TagResourceOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) TagResourceWithContext(aws.Context, *cloudwatch.TagResourceInput, ...request.Option) (*cloudwatch.TagResourceOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) TagResourceRequest(*cloudwatch.TagResourceInput) (*request.Request, *cloudwatch.TagResourceOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) UntagResource(*cloudwatch.UntagResourceInput) (*cloudwatch.UntagResourceOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) UntagResourceWithContext(aws.Context, *cloudwatch.UntagResourceInput, ...request.Option) (*cloudwatch.UntagResourceOutput, error) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) UntagResourceRequest(*cloudwatch.UntagResourceInput) (*request.Request, *cloudwatch.UntagResourceOutput) {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) WaitUntilAlarmExists(*cloudwatch.DescribeAlarmsInput) error {
	panic("implement me")
}

func (f *fakeAWSCloudWatchClient) WaitUntilAlarmExistsWithContext(aws.Context, *cloudwatch.DescribeAlarmsInput, ...request.WaiterOption) error {
	panic("implement me")
}
