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

	"github.com/loadimpact/k6/stats"

	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v4"
)

func TestInitWithoutAddressErrors(t *testing.T) {
	var c = &Collector{
		Config: Config{},
		Type:   "testtype",
	}
	err := c.Init()
	require.Error(t, err)
}

func TestInitWithBogusAddressErrors(t *testing.T) {
	var c = &Collector{
		Config: Config{
			Addr: null.StringFrom("localhost:90000"),
		},
		Type: "testtype",
	}
	err := c.Init()
	require.Error(t, err)
}

func TestLinkReturnAddress(t *testing.T) {
	var bogusValue = "bogus value"
	var c = &Collector{
		Config: Config{
			Addr: null.StringFrom(bogusValue),
		},
	}
	require.Equal(t, bogusValue, c.Link())
}

func TestGetRequiredSystemTags(t *testing.T) {
	var c = &Collector{}
	require.Equal(t, stats.SystemTagSet(0), c.GetRequiredSystemTags())
}
