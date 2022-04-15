package remotewrite

import (
	"fmt"
	"testing"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

func TestTagsToLabels(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		tags   *metrics.SampleTags
		config Config
		labels []prompb.Label
	}{
		"empty-tags": {
			tags: &metrics.SampleTags{},
			config: Config{
				KeepTags:    null.BoolFrom(true),
				KeepNameTag: null.BoolFrom(false),
			},
			labels: []prompb.Label{},
		},
		"name-tag-discard": {
			tags: metrics.NewSampleTags(map[string]string{"foo": "bar", "name": "nnn"}),
			config: Config{
				KeepTags:    null.BoolFrom(true),
				KeepNameTag: null.BoolFrom(false),
			},
			labels: []prompb.Label{
				{Name: "foo", Value: "bar"},
			},
		},
		"name-tag-keep": {
			tags: metrics.NewSampleTags(map[string]string{"foo": "bar", "name": "nnn"}),
			config: Config{
				KeepTags:    null.BoolFrom(true),
				KeepNameTag: null.BoolFrom(true),
			},
			labels: []prompb.Label{
				{Name: "foo", Value: "bar"},
				{Name: "name", Value: "nnn"},
			},
		},
		"url-tag-discard": {
			tags: metrics.NewSampleTags(map[string]string{"foo": "bar", "url": "uuu"}),
			config: Config{
				KeepTags:   null.BoolFrom(true),
				KeepUrlTag: null.BoolFrom(false),
			},
			labels: []prompb.Label{
				{Name: "foo", Value: "bar"},
			},
		},
		"url-tag-keep": {
			tags: metrics.NewSampleTags(map[string]string{"foo": "bar", "url": "uuu"}),
			config: Config{
				KeepTags:   null.BoolFrom(true),
				KeepUrlTag: null.BoolFrom(true),
			},
			labels: []prompb.Label{
				{Name: "foo", Value: "bar"},
				{Name: "url", Value: "uuu"},
			},
		},
		"discard-tags": {
			tags: metrics.NewSampleTags(map[string]string{"foo": "bar", "name": "nnn"}),
			config: Config{
				KeepTags: null.BoolFrom(false),
			},
			labels: []prompb.Label{},
		},
	}

	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			labels, err := tagsToLabels(testCase.tags, testCase.config)
			require.NoError(t, err)

			assert.Equal(t, len(testCase.labels), len(labels))

			for i := range testCase.labels {
				var found bool

				// order is not guaranteed ATM
				for j := range labels {
					if labels[j].Name == testCase.labels[i].Name {
						assert.Equal(t, testCase.labels[i].Value, labels[j].Value)
						found = true
						break
					}

				}
				if !found {
					assert.Fail(t, fmt.Sprintf("Not found label %s: \n"+
						"expected: %v\n"+
						"actual  : %v", testCase.labels[i].Name, testCase.labels, labels))
				}
			}
		})
	}
}
