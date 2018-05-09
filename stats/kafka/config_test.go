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
	c := &Config{}
	err := c.UnmarshalText([]byte("broker=broker1,topic=someTopic,format=influx"))
	assert.Nil(t, err)
	assert.Equal(t, c.Broker, "broker1")
	assert.Equal(t, c.Topic, "someTopic")
	assert.Equal(t, c.Format, "influx")
}
