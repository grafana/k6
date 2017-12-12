/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package statsd

import (
	"fmt"
	"strings"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
)

const (
	connStrSplitter   = ":"
	defaultBufferSize = 10
)

// MakeClient creates a new statsd buffered client
func MakeClient(conf Config, clientType ClientType) (*statsd.Client, error) {
	connStr := ""
	bufferSize := defaultBufferSize
	namespace := ""

	switch clientType {
	case StatsD:
		connStr = fmt.Sprintf("%s%s%s", conf.StatsDAddr, connStrSplitter, conf.StatsDPort)
		bufferSize = conf.StatsDBufferSize
	case DogStatsD:
		connStr = fmt.Sprintf("%s%s%s", conf.DogStatsDAddr, connStrSplitter, conf.DogStatsDPort)
		bufferSize = conf.DogStatsDBufferSize
		namespace = conf.DogStatsNamespace
	}
	log.
		WithField("type", clientType.String()).
		Debugf("Connecting to %s metrics server: %s", clientType.String(), connStr)

	if !validConnStr(connStr) {
		return nil, fmt.Errorf("%s: connection string is invalid. Received: \"%+v\"", clientType.String(), connStr)
	}

	c, err := statsd.NewBuffered(connStr, bufferSize)
	if err != nil {
		log.Info(err)
		return nil, err
	}

	if clientType == DogStatsD {
		c.Namespace = namespace
	}

	return c, nil
}

func generateDataPoint(sample stats.Sample) *Sample {
	return &Sample{
		Type:   sample.Metric.Type,
		Metric: sample.Metric.Name,
		Data: SampleData{
			Time:  sample.Time,
			Value: sample.Value,
			Tags:  sample.Tags,
		},
	}
}

func validConnStr(connStr string) bool {
	sVal := strings.Split(connStr, connStrSplitter)
	if sVal[0] == "" || sVal[1] == "" {
		return false
	}
	return true
}

func mapToSlice(tags map[string]string) []string {
	res := []string{}
	for key, value := range tags {
		if value != "" {
			res = append(res, fmt.Sprintf("%s:%v", key, value))
		}
	}
	return res
}

func takeOnly(tags map[string]string, whitelist string) map[string]string {
	res := map[string]string{}
	for key, value := range tags {
		if strings.Contains(whitelist, key) && value != "" {
			res[key] = value
		}
	}
	return res
}
