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

package influxdb

import (
	"testing"

	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/stretchr/testify/assert"
	null "gopkg.in/guregu/null.v4"
)

func TestMakeBatchConfig(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		assert.Equal(t,
			client.BatchPointsConfig{Database: "k6"},
			MakeBatchConfig(Config{}),
		)
	})
	t.Run("DB Set", func(t *testing.T) {
		assert.Equal(t,
			client.BatchPointsConfig{Database: "dbname"},
			MakeBatchConfig(Config{DB: null.StringFrom("dbname")}),
		)
	})
}
