package awscloudwatch

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type client struct {
	*cloudwatch.CloudWatch
	namespace string
}

// NewClient creates a new client with the CloudWatch client returned by clientFactory
func NewClient(namespace string, clientFactory func() (*cloudwatch.CloudWatch, error)) (*client, error) {
	svc, err := clientFactory()
	if err != nil {
		return nil, err
	}

	return &client{
		CloudWatch: svc,
		namespace:  namespace,
	}, nil
}

// NewCloudWatchClient creates a new CloudWatch client
func NewCloudWatchClient() (*cloudwatch.CloudWatch, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	return cloudwatch.New(sess), nil
}

const maxMetricsPerCall = 20

// reportSamples reports samples to CloudWatch.
// It send samples on batches of max 20, which is the upper limit of metrics
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
			logrus.WithError(err).Debug("Error sending metrics to CloudWatch")
			lastError = err
		}
	}

	if lastError != nil {
		return errors.Wrap(lastError, "Error sending metrics, activate debug to see individual one")
	}

	return nil
}

func (c *client) address() string {
	return c.ClientInfo.Endpoint
}

func toMetricDatum(s *sample) *cloudwatch.MetricDatum {
	datum := &cloudwatch.MetricDatum{
		Value:      &s.Value,
		MetricName: &s.Metric,
		Timestamp:  &s.Time,
	}

	var dims []*cloudwatch.Dimension

	for name, value := range s.Tags {
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
