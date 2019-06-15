package awscloudwatch

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type client struct {
	cloudwatchiface.CloudWatchAPI
	namespace string
	endpoint  string
}

// ClientFactory returns a function that creates the AWS CloudWatch client
func ClientFactory(namespace string) func() (cloudWatchClient, error) {
	return func() (cloudWatchClient, error) {
		cw, err := newCloudWatchClient()
		if err != nil {
			return nil, err
		}

		return &client{
			CloudWatchAPI: cw,
			namespace:     namespace,
			endpoint:      cw.Endpoint,
		}, nil
	}
}

// newCloudWatchClient creates a new CloudWatch client
func newCloudWatchClient() (*cloudwatch.CloudWatch, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	return cloudwatch.New(sess), nil
}

const maxMetricsPerCall = 20

// reportSamples reports samples to CloudWatch.
// It sends samples in batches of max 20, which is the upper limit of metrics
// accepted per request by CloudWatch
func (c *client) reportSamples(samples []*sample) error {
	samplesSent := 0
	var lastError error

	for samplesSent < len(samples) {
		input := &cloudwatch.PutMetricDataInput{Namespace: &c.namespace}
		upperLimit := samplesSent + maxMetricsPerCall
		if len(samples) < upperLimit {
			upperLimit = len(samples)
		}

		for _, s := range samples[samplesSent:upperLimit] {
			input.MetricData = append(input.MetricData, toMetricDatum(s))
			samplesSent++
		}

		_, err := c.PutMetricData(input)
		if err != nil {
			logrus.WithError(err).Warn("Error sending metrics to CloudWatch")
			lastError = err
		}
	}

	if lastError != nil {
		return errors.Wrap(lastError, "Error sending metrics")
	}

	return nil
}

func (c *client) address() string {
	return c.endpoint
}

const maxNumberOfDimensions = 10

func toMetricDatum(s *sample) *cloudwatch.MetricDatum {
	datum := &cloudwatch.MetricDatum{
		Value:      &s.Value,
		MetricName: &s.Metric,
		Timestamp:  &s.Time,
	}

	var dims []*cloudwatch.Dimension

	for name, value := range s.Tags {
		if len(dims) == maxNumberOfDimensions {
			logrus.WithField("tags", s.Tags).
				WithField("dimensions_included", dims).
				Warnf("More than 10 tags, just 10 will be reported to CloudWatch")
			break
		}

		if value != "" {
			dims = append(dims, &cloudwatch.Dimension{
				Name:  aws.String(name),
				Value: aws.String(value),
			})
		}
	}

	if len(dims) > 0 {
		datum.Dimensions = dims
	}

	return datum
}
