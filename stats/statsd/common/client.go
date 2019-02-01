/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package common

import (
	"fmt"

	"github.com/DataDog/datadog-go/statsd"
	log "github.com/sirupsen/logrus"
)

// ClientType defines a statsd client type
type ClientType int

func (t ClientType) String() string {
	switch t {
	case StatsD:
		return "StatsD"
	case Datadog:
		return "Datadog"
	default:
		return "[INVALID]"
	}
}

// Possible values for ClientType
const (
	StatsD = ClientType(iota)
	Datadog
)

// MakeClient creates a new statsd buffered generic client
func MakeClient(conf Config, cliType ClientType) (*statsd.Client, error) {
	if conf.Addr.String == "" {
		return nil, fmt.Errorf(
			"%s: connection string is invalid. Received: \"%+s\"",
			cliType, conf.Addr.String,
		)
	}

	c, err := statsd.NewBuffered(conf.Addr.String, int(conf.BufferSize.Int64))
	if err != nil {
		log.Info(err)
		return nil, err
	}
	if namespace := conf.Namespace.String; namespace != "" {
		c.Namespace = namespace
	}

	return c, nil
}
