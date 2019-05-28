package awscloudwatch

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCloudWatchClient(t *testing.T) {
	s := &sample{
		Tags: map[string]string{"foo": "bar", "proto": "http"},
	}

	require.Equal(
		t, []*cloudwatch.Dimension{{Name: aws.String("foo"), Value: aws.String("bar")}},
		toMetricDatum(s).Dimensions,
	)
}
