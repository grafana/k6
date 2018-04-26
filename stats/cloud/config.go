/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cloud

import (
	"time"

	"github.com/loadimpact/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

// Config holds all the necessary data and options for sending metrics to the Load Impact cloud.
type Config struct {
	Token           null.String `json:"token" envconfig:"CLOUD_TOKEN"`
	DeprecatedToken null.String `json:"-" envconfig:"K6CLOUD_TOKEN"`
	Name            null.String `json:"name" envconfig:"CLOUD_NAME"`
	Host            null.String `json:"host" envconfig:"CLOUD_HOST"`
	WebAppURL       null.String `json:"webAppURL" envconfig:"CLOUD_WEB_APP_URL"`
	NoCompress      null.Bool   `json:"noCompress" envconfig:"CLOUD_NO_COMPRESS"`
	ProjectID       null.Int    `json:"projectID" envconfig:"CLOUD_PROJECT_ID"`

	// The time interval between periodic API calls for sending samples to the cloud ingest service.
	MetricPushInterval types.NullDuration `json:"metricPushInterval" envconfig:"CLOUD_METRIC_PUSH_INTERVAL"`

	// If specified and greater than 0, sample aggregation with that period is enabled:
	// - HTTP trail samples will be collected separately and not
	//   included in the default sample buffer that's directly sent
	//   to the cloud service every MetricPushInterval.
	// - Every AggregationCalcInterval, all collected HTTP Trails will be
	//   split into AggregationPeriod-sized time buckets (time slots) and
	//   then into sub-buckets according to their tags (each sub-bucket
	//   will contain only HTTP trails with the same sample tags).
	// - If AggregationWaitPeriod is not passed for a particular time
	//   bucket, it's left undisturbed until the next AggregationCalcInterval
	//   tick comes along.
	// - If AggregationWaitPeriod is passed for a time bucket, all of its
	//   sub-buckets are traversed:
	//     - Any sub-buckets that have less than AggregationMinSamples HTTP
	//       trails in them are not aggregated, instead the HTTP trails are
	//       just added to the default sample buffer.
	//     - Sub-buckets with at least AggregationMinSamples HTTP trails
	//       are aggregated. The HTTP trails are checked for outliers
	//       (Trails with metrics outside of the AggregationOutliers) and
	//       all non-outliers are aggregated. The aggregation result and all
	//       found outliers are then added to the default sample buffer for
	//       sending to the cloud ingest service on the next MetricPushInterval.
	AggregationPeriod types.NullDuration `json:"aggregationPeriod" envconfig:"CLOUD_AGGREGATION_PERIOD"`

	// If aggregation is enabled, this is how often new HTTP trails will be sorted into buckets and sub-buckets and aggregated.
	AggregationCalcInterval types.NullDuration `json:"aggregationCalcInterval" envconfig:"CLOUD_AGGREGATION_CALC_INTERVAL"`

	// If aggregation is enabled, this specifies how long we'll wait for period samples to accumulate before trying to aggregate them.
	AggregationWaitPeriod types.NullDuration `json:"aggregationWaitPeriod" envconfig:"CLOUD_AGGREGATION_WAIT_PERIOD"`

	// If aggregation is enabled, but the collected samples for a certain AggregationPeriod after AggregationPushDelay has passed are less than this number, they won't be aggregated.
	AggregationMinSamples null.Int `json:"aggregationMinSamples" envconfig:"CLOUD_AGGREGATION_MIN_SAMPLES"`

	// Which HTTP trails (how many IQRs below Q1 or above Q3) to consier non-aggregatable outliers.
	AggregationOutliers null.Float `json:"aggregationOutliers" envconfig:"CLOUD_AGGREGATION_OUTLIERS"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		Host:                    null.StringFrom("https://ingest.loadimpact.com"),
		WebAppURL:               null.StringFrom("https://app.loadimpact.com"),
		MetricPushInterval:      types.NullDurationFrom(1 * time.Second),
		AggregationPeriod:       types.NullDurationFrom(1 * time.Second),
		AggregationCalcInterval: types.NullDurationFrom(3 * time.Second),
		AggregationWaitPeriod:   types.NullDurationFrom(5 * time.Second),
		AggregationMinSamples:   null.IntFrom(100),
		AggregationOutliers:     null.FloatFrom(1.5),
	}
}

// Apply saves config non-zero config values from the passed config in the receiver.
func (c Config) Apply(cfg Config) Config {
	if cfg.Token.Valid {
		c.Token = cfg.Token
	}
	if cfg.DeprecatedToken.Valid {
		c.DeprecatedToken = cfg.DeprecatedToken
	}
	if cfg.Name.Valid {
		c.Name = cfg.Name
	}
	if cfg.Host.Valid {
		c.Host = cfg.Host
	}
	if cfg.NoCompress.Valid {
		c.NoCompress = cfg.NoCompress
	}
	if cfg.ProjectID.Valid {
		c.ProjectID = cfg.ProjectID
	}
	if cfg.MetricPushInterval.Valid {
		c.MetricPushInterval = cfg.MetricPushInterval
	}
	if cfg.AggregationPeriod.Valid {
		c.AggregationPeriod = cfg.AggregationPeriod
	}
	if cfg.AggregationCalcInterval.Valid {
		c.AggregationCalcInterval = cfg.AggregationCalcInterval
	}
	if cfg.AggregationWaitPeriod.Valid {
		c.AggregationWaitPeriod = cfg.AggregationWaitPeriod
	}
	if cfg.AggregationMinSamples.Valid {
		c.AggregationMinSamples = cfg.AggregationMinSamples
	}
	if cfg.AggregationOutliers.Valid {
		c.AggregationOutliers = cfg.AggregationOutliers
	}
	return c
}
