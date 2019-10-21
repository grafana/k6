/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
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

package ui

import (
	"testing"

	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
)

var verifyTests = []struct {
	in  string
	out error
}{
	{"avg", nil},
	{"min", nil},
	{"med", nil},
	{"max", nil},
	{"p(0)", nil},
	{"p(90)", nil},
	{"p(95)", nil},
	{"p(99)", nil},
	{"p(99.9)", nil},
	{"p(99.9999)", nil},
	{"nil", ErrStatUnknownFormat},
	{" avg", ErrStatUnknownFormat},
	{"avg ", ErrStatUnknownFormat},
	{"", ErrStatEmptyString},
}

var defaultTrendColumns = TrendColumns

func createTestTrendSink(count int) *stats.TrendSink {
	sink := stats.TrendSink{}

	for i := 0; i < count; i++ {
		sink.Add(stats.Sample{Value: float64(i)})
	}

	return &sink
}

func TestVerifyTrendColumnStat(t *testing.T) {
	for _, testCase := range verifyTests {
		err := VerifyTrendColumnStat(testCase.in)
		assert.Equal(t, testCase.out, err)
	}
}

func TestUpdateTrendColumns(t *testing.T) {
	sink := createTestTrendSink(100)

	t.Run("No stats", func(t *testing.T) {
		TrendColumns = defaultTrendColumns

		UpdateTrendColumns(make([]string, 0))

		assert.Equal(t, defaultTrendColumns, TrendColumns)
	})

	t.Run("One stat", func(t *testing.T) {
		TrendColumns = defaultTrendColumns

		UpdateTrendColumns([]string{"avg"})

		assert.Exactly(t, 1, len(TrendColumns))
		assert.Exactly(t,
			sink.Avg,
			TrendColumns[0].Get(sink))
	})

	t.Run("Multiple stats", func(t *testing.T) {
		TrendColumns = defaultTrendColumns

		UpdateTrendColumns([]string{"med", "max"})

		assert.Exactly(t, 2, len(TrendColumns))
		assert.Exactly(t, sink.Med, TrendColumns[0].Get(sink))
		assert.Exactly(t, sink.Max, TrendColumns[1].Get(sink))
	})

	t.Run("Ignore invalid stats", func(t *testing.T) {
		TrendColumns = defaultTrendColumns

		UpdateTrendColumns([]string{"med", "max", "invalid"})

		assert.Exactly(t, 2, len(TrendColumns))
		assert.Exactly(t, sink.Med, TrendColumns[0].Get(sink))
		assert.Exactly(t, sink.Max, TrendColumns[1].Get(sink))
	})

	t.Run("Percentile stats", func(t *testing.T) {
		TrendColumns = defaultTrendColumns

		UpdateTrendColumns([]string{"p(99.9999)"})

		assert.Exactly(t, 1, len(TrendColumns))
		assert.Exactly(t, sink.P(0.999999), TrendColumns[0].Get(sink))
	})
}

func TestGeneratePercentileTrendColumn(t *testing.T) {
	sink := createTestTrendSink(100)

	t.Run("Happy path", func(t *testing.T) {
		colFunc, err := generatePercentileTrendColumn("p(99)")
		assert.Nil(t, err)
		assert.NotNil(t, colFunc)
		assert.Exactly(t, sink.P(0.99), colFunc(sink))
		assert.NotEqual(t, sink.P(0.98), colFunc(sink))
		assert.Nil(t, err)
	})

	t.Run("Empty stat", func(t *testing.T) {
		colFunc, err := generatePercentileTrendColumn("")

		assert.Nil(t, colFunc)
		assert.Exactly(t, err, ErrStatEmptyString)
	})

	t.Run("Invalid format", func(t *testing.T) {
		colFunc, err := generatePercentileTrendColumn("p90")

		assert.Nil(t, colFunc)
		assert.Exactly(t, err, ErrStatUnknownFormat)
	})

	t.Run("Invalid format 2", func(t *testing.T) {
		colFunc, err := generatePercentileTrendColumn("p(90")

		assert.Nil(t, colFunc)
		assert.Exactly(t, err, ErrStatUnknownFormat)
	})

	t.Run("Invalid float", func(t *testing.T) {
		colFunc, err := generatePercentileTrendColumn("p(a)")

		assert.Nil(t, colFunc)
		assert.Exactly(t, err, ErrPercentileStatInvalidValue)
	})
}
