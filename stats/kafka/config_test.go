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

package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigUnmarshalText(t *testing.T) {
	c1 := &Config{}

	err := c1.UnmarshalText([]byte("brokers=broker1,topic=someTopic,format=influx"))
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker1"}, c1.Brokers)
	assert.Equal(t, "someTopic", c1.Topic)
	assert.Equal(t, "influx", c1.Format)

	c2 := &Config{}

	err = c2.UnmarshalText([]byte("brokers={broker2,broker3:9092},topic=someTopic2,format=json"))
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker2", "broker3:9092"}, c2.Brokers)
	assert.Equal(t, "someTopic2", c2.Topic)
	assert.Equal(t, "json", c2.Format)
}
