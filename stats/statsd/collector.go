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
	"github.com/loadimpact/k6/stats/statsd/common"
	log "github.com/sirupsen/logrus"
)

// New creates a new statsd connector client
func New(conf common.Config) (*common.Collector, error) {
	cl, err := common.MakeClient(conf, common.StatsD)
	if err != nil {
		return nil, err
	}

	return &common.Collector{
		Client: cl,
		Config: conf,
		Logger: log.WithField("type", common.StatsD.String()),
		Type:   common.StatsD,
	}, nil
}
