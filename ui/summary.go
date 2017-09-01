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
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
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
	StdColor      = color.New()                          // Default color.
	SuccColor     = color.New(color.FgGreen)             // Successful stuff.
	FailColor     = color.New(color.FgRed)               // Failed stuff.
	GrayColor     = color.New(color.Faint)               // Padding and disabled stuff.
	ValueColor    = color.New(color.FgCyan)              // Values of all kinds.
	ExtraColor    = color.New(color.FgCyan, color.Faint) // Extra annotations for values.
	ExtraKeyColor = color.New(color.Faint)               // Keys inside extra annotations.

	TrendColumns = []TrendColumn{
		{"avg", func(s *stats.TrendSink) float64 { return s.Avg }},
		{"min", func(s *stats.TrendSink) float64 { return s.Min }},
		{"med", func(s *stats.TrendSink) float64 { return s.Med }},
		{"max", func(s *stats.TrendSink) float64 { return s.Max }},
		{"p(90)", func(s *stats.TrendSink) float64 { return s.P(0.90) }},
		{"p(95)", func(s *stats.TrendSink) float64 { return s.P(0.95) }},
	}
)

type TrendColumn struct {
	Key string
	Get func(s *stats.TrendSink) float64
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
	sort.Strings(checkNames)
	for _, name := range checkNames {
		SummarizeCheck(w, indent, group.Checks[name])
	}
	if len(checkNames) > 0 {
		fmt.Fprintf(w, "\n")
	}

	var groupNames []string
	for _, grp := range group.Groups {
		groupNames = append(groupNames, grp.Name)
	}
	sort.Strings(groupNames)
	for _, name := range groupNames {
		SummarizeGroup(w, indent, group.Groups[name])
	}
}

func NonTrendMetricValueForSum(t time.Duration, m *stats.Metric) (data string, extra []string) {
	m.Sink.Calc()
	switch sink := m.Sink.(type) {
	case *stats.CounterSink:
		value := m.HumanizeValue(sink.Value)
		rate := m.HumanizeValue(sink.Value / float64(t/time.Second))
		return value, []string{rate + "/s"}
	case *stats.GaugeSink:
		value := m.HumanizeValue(sink.Value)
		min := m.HumanizeValue(sink.Min)
		max := m.HumanizeValue(sink.Max)
		return value, []string{"min=" + min, "max=" + max}
	case *stats.RateSink:
		value := m.HumanizeValue(float64(sink.Trues) / float64(sink.Total))
		passes := sink.Trues
		fails := sink.Total - sink.Trues
		return value, []string{"✓ " + strconv.FormatInt(passes, 10), "✗ " + strconv.FormatInt(fails, 10)}
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

func SummarizeMetrics(w io.Writer, indent string, t time.Duration, metrics map[string]*stats.Metric) {
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

		if sink, ok := m.Sink.(*stats.TrendSink); ok {
			cols := make([]string, len(TrendColumns))
			for i, col := range TrendColumns {
				value := m.HumanizeValue(col.Get(sink))
				if l := StrWidth(value); l > trendColMaxLens[i] {
					trendColMaxLens[i] = l
				}
				cols[i] = value
			}
			trendCols[name] = cols
			continue
		}

		value, extra := NonTrendMetricValueForSum(t, m)
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

		fmtData := ""
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
		fmt.Fprint(w, indent+fmtIndent+markColor.Sprint(mark)+" "+fmtName+" "+fmtData+"\n")
	}
}

// Summarizes a dataset and returns whether the test run was considered a success.
func Summarize(w io.Writer, indent string, data SummaryData) {
	SummarizeGroup(w, indent+"    ", data.Root)
	SummarizeMetrics(w, indent+"  ", data.Time, data.Metrics)
}
