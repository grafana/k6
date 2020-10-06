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
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

type config struct {
	addr, namespace null.String
	bufferSize      null.Int
	pushInterval    types.NullDuration
}

func (c config) GetAddr() null.String {
	return c.addr
}

func (c config) GetBufferSize() null.Int {
	return c.bufferSize
}

func (c config) GetNamespace() null.String {
	return c.namespace
}

func (c config) GetPushInterval() types.NullDuration {
	return c.pushInterval
}

func TestInitWithoutAddressErrors(t *testing.T) {
	c := &Collector{
		Config: config{},
		Type:   "testtype",
		Logger: testutils.NewLogger(t),
	}
	err := c.Init()
	require.Error(t, err)
}

func TestInitWithBogusAddressErrors(t *testing.T) {
	c := &Collector{
		Config: config{
			addr: null.StringFrom("localhost:90000"),
		},
		Type:   "testtype",
		Logger: testutils.NewLogger(t),
	}
	err := c.Init()
	require.Error(t, err)
}

func TestLinkReturnAddress(t *testing.T) {
	bogusValue := "bogus value"
	c := &Collector{
		Config: config{
			addr: null.StringFrom(bogusValue),
		},
	}
	require.Equal(t, bogusValue, c.Link())
}

func TestGetRequiredSystemTags(t *testing.T) {
	c := &Collector{}
	require.Equal(t, stats.SystemTagSet(0), c.GetRequiredSystemTags())
}
