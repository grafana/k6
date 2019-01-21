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

package ui

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"golang.org/x/text/unicode/norm"
)

const (
	GroupPrefix   = "█"
	DetailsPrefix = "↳"

	SuccMark = "✓"
	FailMark = "✗"
)

var (
	ErrStatEmptyString            = errors.New("invalid stat, empty string")
	ErrStatUnknownFormat          = errors.New("invalid stat, unknown format")
	ErrPercentileStatInvalidValue = errors.New("invalid percentile stat value, accepts a number")
)

var TrendColumns = []TrendColumn{
	{"avg", func(s *stats.TrendSink) float64 { return s.Avg }},
	{"min", func(s *stats.TrendSink) float64 { return s.Min }},
	{"med", func(s *stats.TrendSink) float64 { return s.Med }},
	{"max", func(s *stats.TrendSink) float64 { return s.Max }},
	{"p(90)", func(s *stats.TrendSink) float64 { return s.P(0.90) }},
	{"p(95)", func(s *stats.TrendSink) float64 { return s.P(0.95) }},
}

type TrendColumn struct {
	Key string
	Get func(s *stats.TrendSink) float64
}

// VerifyTrendColumnStat checks if stat is a valid trend column
func VerifyTrendColumnStat(stat string) error {
	if stat == "" {
		return ErrStatEmptyString
	}

	for _, col := range TrendColumns {
		if col.Key == stat {
			return nil
		}
	}

	_, err := generatePercentileTrendColumn(stat)
	return err
}

// UpdateTrendColumns updates the default trend columns with user defined ones
func UpdateTrendColumns(stats []string) {
	newTrendColumns := make([]TrendColumn, 0, len(stats))

	for _, stat := range stats {
		percentileTrendColumn, err := generatePercentileTrendColumn(stat)

		if err == nil {
			newTrendColumns = append(newTrendColumns, TrendColumn{stat, percentileTrendColumn})
			continue
		}

		for _, col := range TrendColumns {
			if col.Key == stat {
				newTrendColumns = append(newTrendColumns, col)
				break
			}
		}
	}

	if len(newTrendColumns) > 0 {
		TrendColumns = newTrendColumns
	}
}

func generatePercentileTrendColumn(stat string) (func(s *stats.TrendSink) float64, error) {
	if stat == "" {
		return nil, ErrStatEmptyString
	}

	if !strings.HasPrefix(stat, "p(") || !strings.HasSuffix(stat, ")") {
		return nil, ErrStatUnknownFormat
	}

	percentile, err := strconv.ParseFloat(stat[2:len(stat)-1], 64)

	if err != nil {
		return nil, ErrPercentileStatInvalidValue
	}

	percentile = percentile / 100

	return func(s *stats.TrendSink) float64 { return s.P(percentile) }, nil
}

// Returns the actual width of the string.
func StrWidth(s string) (n int) {
	var it norm.Iter
	it.InitString(norm.NFKD, s)

	inEscSeq := false
	inLongEscSeq := false
	for !it.Done() {
		data := it.Next()

		// Skip over ANSI escape codes.
		if data[0] == '\x1b' {
			inEscSeq = true
			continue
		}
		if inEscSeq && data[0] == '[' {
			inLongEscSeq = true
			continue
		}
		if inEscSeq && inLongEscSeq && data[0] >= 0x40 && data[0] <= 0x7E {
			inEscSeq = false
			inLongEscSeq = false
			continue
		}
		if inEscSeq && !inLongEscSeq && data[0] >= 0x40 && data[0] <= 0x5F {
			inEscSeq = false
			continue
		}

		n++
	}
	return
}

// SummaryData represents data passed to Summarize.
type SummaryData struct {
	Opts    lib.Options
	Root    *lib.Group
	Metrics map[string]*stats.Metric
	Time    time.Duration
}

func SummarizeCheck(w io.Writer, indent string, check *lib.Check) {
	mark := SuccMark
	color := SuccColor
	if check.Fails > 0 {
		mark = FailMark
		color = FailColor
	}
	_, _ = color.Fprintf(w, "%s%s %s\n", indent, mark, check.Name)
	if check.Fails > 0 {
		_, _ = color.Fprintf(w, "%s %s  %d%% — %s %d / %s %d\n",
			indent, DetailsPrefix,
			int(100*(float64(check.Passes)/float64(check.Fails+check.Passes))),
			SuccMark, check.Passes, FailMark, check.Fails,
		)
	}
}

func SummarizeGroup(w io.Writer, indent string, group *lib.Group) {
	if group.Name != "" {
		_, _ = fmt.Fprintf(w, "%s%s %s\n\n", indent, GroupPrefix, group.Name)
		indent = indent + "  "
	}

	var checkNames []string
	for _, check := range group.Checks {
		checkNames = append(checkNames, check.Name)
	}
	for _, name := range checkNames {
		SummarizeCheck(w, indent, group.Checks[name])
	}
	if len(checkNames) > 0 {
		_, _ = fmt.Fprintf(w, "\n")
	}

	var groupNames []string
	for _, grp := range group.Groups {
		groupNames = append(groupNames, grp.Name)
	}
	for _, name := range groupNames {
		SummarizeGroup(w, indent, group.Groups[name])
	}
}

func NonTrendMetricValueForSum(t time.Duration, timeUnit string, m *stats.Metric) (data string, extra []string) {
	switch sink := m.Sink.(type) {
	case *stats.CounterSink:
		value := sink.Value
		rate := 0.0
		if t > 0 {
			rate = value / (float64(t) / float64(time.Second))
		}
		return m.HumanizeValue(value, timeUnit), []string{m.HumanizeValue(rate, timeUnit) + "/s"}
	case *stats.GaugeSink:
		value := sink.Value
		min := sink.Min
		max := sink.Max
		return m.HumanizeValue(value, timeUnit), []string{
			"min=" + m.HumanizeValue(min, timeUnit),
			"max=" + m.HumanizeValue(max, timeUnit),
		}
	case *stats.RateSink:
		value := float64(sink.Trues) / float64(sink.Total)
		passes := sink.Trues
		fails := sink.Total - sink.Trues
		return m.HumanizeValue(value, timeUnit), []string{
			"✓ " + strconv.FormatInt(passes, 10),
			"✗ " + strconv.FormatInt(fails, 10),
		}
	default:
		return "[no data]", nil
	}
}

func DisplayNameForMetric(m *stats.Metric) string {
	if m.Sub.Parent != "" {
		return "{ " + m.Sub.Suffix + " }"
	}
	return m.Name
}

func IndentForMetric(m *stats.Metric) string {
	if m.Sub.Parent != "" {
		return "  "
	}
	return ""
}

func SummarizeMetrics(w io.Writer, indent string, t time.Duration, timeUnit string, metrics map[string]*stats.Metric) {
	names := []string{}
	nameLenMax := 0

	values := make(map[string]string)
	valueMaxLen := 0
	extras := make(map[string][]string)
	extraMaxLens := make([]int, 2)

	trendCols := make(map[string][]string)
	trendColMaxLens := make([]int, len(TrendColumns))

	for name, m := range metrics {
		names = append(names, name)

		// When calculating widths for metrics, account for the indentation on submetrics.
		displayName := DisplayNameForMetric(m) + IndentForMetric(m)
		if l := StrWidth(displayName); l > nameLenMax {
			nameLenMax = l
		}

		m.Sink.Calc()
		if sink, ok := m.Sink.(*stats.TrendSink); ok {
			cols := make([]string, len(TrendColumns))
			for i, col := range TrendColumns {
				value := m.HumanizeValue(col.Get(sink), timeUnit)
				if l := StrWidth(value); l > trendColMaxLens[i] {
					trendColMaxLens[i] = l
				}
				cols[i] = value
			}
			trendCols[name] = cols
			continue
		}

		value, extra := NonTrendMetricValueForSum(t, timeUnit, m)
		values[name] = value
		if l := StrWidth(value); l > valueMaxLen {
			valueMaxLen = l
		}
		extras[name] = extra
		if len(extra) > 1 {
			for i, ex := range extra {
				if l := StrWidth(ex); l > extraMaxLens[i] {
					extraMaxLens[i] = l
				}
			}
		}
	}

	sort.Strings(names)
	tmpCols := make([]string, len(TrendColumns))
	for _, name := range names {
		m := metrics[name]

		mark := " "
		markColor := StdColor
		if m.Tainted.Valid {
			if m.Tainted.Bool {
				mark = FailMark
				markColor = FailColor
			} else {
				mark = SuccMark
				markColor = SuccColor
			}
		}

		fmtName := DisplayNameForMetric(m)
		fmtIndent := IndentForMetric(m)
		fmtName += GrayColor.Sprint(strings.Repeat(".", nameLenMax-StrWidth(fmtName)-StrWidth(fmtIndent)+3) + ":")

		var fmtData string
		if cols := trendCols[name]; cols != nil {
			for i, val := range cols {
				tmpCols[i] = TrendColumns[i].Key + "=" + ValueColor.Sprint(val) + strings.Repeat(" ", trendColMaxLens[i]-StrWidth(val))
			}
			fmtData = strings.Join(tmpCols, " ")
		} else {
			value := values[name]
			fmtData = ValueColor.Sprint(value) + strings.Repeat(" ", valueMaxLen-StrWidth(value))

			extra := extras[name]
			switch len(extra) {
			case 0:
			case 1:
				fmtData = fmtData + " " + ExtraColor.Sprint(extra[0])
			default:
				parts := make([]string, len(extra))
				for i, ex := range extra {
					parts[i] = ExtraColor.Sprint(ex) + strings.Repeat(" ", extraMaxLens[i]-StrWidth(ex))
				}
				fmtData = fmtData + " " + ExtraColor.Sprint(strings.Join(parts, " "))
			}
		}
		_, _ = fmt.Fprint(w, indent+fmtIndent+markColor.Sprint(mark)+" "+fmtName+" "+fmtData+"\n")
	}
}

// Summarizes a dataset and returns whether the test run was considered a success.
func Summarize(w io.Writer, indent string, data SummaryData) {
	if data.Root != nil {
		SummarizeGroup(w, indent+"    ", data.Root)
	}
	SummarizeMetrics(w, indent+"  ", data.Time, data.Opts.SummaryTimeUnit.String, data.Metrics)
}
