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
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
	"golang.org/x/text/unicode/norm"
)

const (
	groupPrefix   = "█"
	detailsPrefix = "↳"

	succMark = "✓"
	failMark = "✗"
)

//nolint: gochecknoglobals
var (
	errStatEmptyString            = errors.New("invalid stat, empty string")
	errStatUnknownFormat          = errors.New("invalid stat, unknown format")
	errPercentileStatInvalidValue = errors.New(
		"invalid percentile stat value, accepts a number between 0 and 100")
	staticResolvers = map[string]func(s *stats.TrendSink) interface{}{
		"avg":   func(s *stats.TrendSink) interface{} { return s.Avg },
		"min":   func(s *stats.TrendSink) interface{} { return s.Min },
		"med":   func(s *stats.TrendSink) interface{} { return s.Med },
		"max":   func(s *stats.TrendSink) interface{} { return s.Max },
		"count": func(s *stats.TrendSink) interface{} { return s.Count },
	}
)

// ErrInvalidStat represents an invalid trend column stat
type ErrInvalidStat struct {
	name string
	err  error
}

func (e ErrInvalidStat) Error() string {
	return errors.Wrapf(e.err, "'%s'", e.name).Error()
}

// Summary handles test summary output
type Summary struct {
	trendColumns        []string
	trendValueResolvers map[string]func(s *stats.TrendSink) interface{}
}

// NewSummary returns a new Summary instance, used for writing a
// summary/report of the test metrics data.
func NewSummary(cols []string) *Summary {
	s := Summary{trendColumns: cols, trendValueResolvers: staticResolvers}

	customResolvers := s.generateCustomTrendValueResolvers(cols)
	for name, res := range customResolvers {
		s.trendValueResolvers[name] = res
	}

	return &s
}

func (s *Summary) generateCustomTrendValueResolvers(cols []string) map[string]func(s *stats.TrendSink) interface{} {
	resolvers := make(map[string]func(s *stats.TrendSink) interface{})

	for _, stat := range cols {
		if _, exists := s.trendValueResolvers[stat]; !exists {
			percentile, err := validatePercentile(stat)
			if err == nil {
				resolvers[stat] = func(s *stats.TrendSink) interface{} { return s.P(percentile / 100) }
			}
		}
	}

	return resolvers
}

// ValidateSummary checks if passed trend columns are valid for use in
// the summary output.
func ValidateSummary(trendColumns []string) error {
	for _, stat := range trendColumns {
		if stat == "" {
			return ErrInvalidStat{stat, errStatEmptyString}
		}

		if _, exists := staticResolvers[stat]; exists {
			continue
		}

		if _, err := validatePercentile(stat); err != nil {
			return ErrInvalidStat{stat, err}
		}
	}

	return nil
}

func validatePercentile(stat string) (float64, error) {
	if !strings.HasPrefix(stat, "p(") || !strings.HasSuffix(stat, ")") {
		return 0, errStatUnknownFormat
	}

	percentile, err := strconv.ParseFloat(stat[2:len(stat)-1], 64)

	if err != nil || ((0 > percentile) || (percentile > 100)) {
		return 0, errPercentileStatInvalidValue
	}

	return percentile, nil
}

// StrWidth returns the actual width of the string.
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

func summarizeCheck(w io.Writer, indent string, check *lib.Check) {
	mark := succMark
	color := SuccColor
	if check.Fails > 0 {
		mark = failMark
		color = FailColor
	}
	_, _ = color.Fprintf(w, "%s%s %s\n", indent, mark, check.Name)
	if check.Fails > 0 {
		_, _ = color.Fprintf(w, "%s %s  %d%% — %s %d / %s %d\n",
			indent, detailsPrefix,
			int(100*(float64(check.Passes)/float64(check.Fails+check.Passes))),
			succMark, check.Passes, failMark, check.Fails,
		)
	}
}

func summarizeGroup(w io.Writer, indent string, group *lib.Group) {
	if group.Name != "" {
		_, _ = fmt.Fprintf(w, "%s%s %s\n\n", indent, groupPrefix, group.Name)
		indent = indent + "  "
	}

	var checkNames []string
	for _, check := range group.Checks {
		checkNames = append(checkNames, check.Name)
	}
	for _, name := range checkNames {
		summarizeCheck(w, indent, group.Checks[name])
	}
	if len(checkNames) > 0 {
		_, _ = fmt.Fprintf(w, "\n")
	}

	var groupNames []string
	for _, grp := range group.Groups {
		groupNames = append(groupNames, grp.Name)
	}
	for _, name := range groupNames {
		summarizeGroup(w, indent, group.Groups[name])
	}
}

func nonTrendMetricValueForSum(t time.Duration, timeUnit string, m *stats.Metric) (data string, extra []string) {
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

func displayNameForMetric(m *stats.Metric) string {
	if m.Sub.Parent != "" {
		return "{ " + m.Sub.Suffix + " }"
	}
	return m.Name
}

func indentForMetric(m *stats.Metric) string {
	if m.Sub.Parent != "" {
		return "  "
	}
	return ""
}

// nolint:funlen
func (s *Summary) summarizeMetrics(w io.Writer, indent string, t time.Duration,
	timeUnit string, metrics map[string]*stats.Metric) {
	names := []string{}
	nameLenMax := 0

	values := make(map[string]string)
	valueMaxLen := 0
	extras := make(map[string][]string)
	extraMaxLens := make([]int, 2)

	trendCols := make(map[string][]string)
	trendColMaxLens := make([]int, len(s.trendColumns))

	for name, m := range metrics {
		names = append(names, name)

		// When calculating widths for metrics, account for the indentation on submetrics.
		displayName := displayNameForMetric(m) + indentForMetric(m)
		if l := StrWidth(displayName); l > nameLenMax {
			nameLenMax = l
		}

		m.Sink.Calc()
		if sink, ok := m.Sink.(*stats.TrendSink); ok {
			cols := make([]string, len(s.trendColumns))

			for i, tc := range s.trendColumns {
				var value string

				resolver := s.trendValueResolvers[tc]

				switch v := resolver(sink).(type) {
				case float64:
					value = m.HumanizeValue(v, timeUnit)
				case uint64:
					value = strconv.FormatUint(v, 10)
				}
				if l := StrWidth(value); l > trendColMaxLens[i] {
					trendColMaxLens[i] = l
				}
				cols[i] = value
			}
			trendCols[name] = cols
			continue
		}

		value, extra := nonTrendMetricValueForSum(t, timeUnit, m)
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

	tmpCols := make([]string, len(s.trendColumns))
	for _, name := range names {
		m := metrics[name]

		mark := " "
		markColor := StdColor
		if m.Tainted.Valid {
			if m.Tainted.Bool {
				mark = failMark
				markColor = FailColor
			} else {
				mark = succMark
				markColor = SuccColor
			}
		}

		fmtName := displayNameForMetric(m)
		fmtIndent := indentForMetric(m)
		fmtName += GrayColor.Sprint(strings.Repeat(".", nameLenMax-StrWidth(fmtName)-StrWidth(fmtIndent)+3) + ":")

		var fmtData string
		if cols := trendCols[name]; cols != nil {
			for i, val := range cols {
				tmpCols[i] = s.trendColumns[i] + "=" + ValueColor.Sprint(val) +
					strings.Repeat(" ", trendColMaxLens[i]-StrWidth(val))
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

// SummaryData represents data passed to Summary.SummarizeMetrics
type SummaryData struct {
	Metrics   map[string]*stats.Metric
	RootGroup *lib.Group
	Time      time.Duration
	TimeUnit  string
}

// SummarizeMetrics creates a summary of provided metrics and writes it to w.
func (s *Summary) SummarizeMetrics(w io.Writer, indent string, data SummaryData) {
	if data.RootGroup != nil {
		summarizeGroup(w, indent+"    ", data.RootGroup)
	}

	s.summarizeMetrics(w, indent+"  ", data.Time, data.TimeUnit, data.Metrics)
}

// SummarizeMetricsJSON summarizes a dataset in JSON format.
func (s *Summary) SummarizeMetricsJSON(w io.Writer, data SummaryData) error {
	m := make(map[string]interface{})
	m["root_group"] = data.RootGroup

	metricsData := make(map[string]interface{})
	for name, m := range data.Metrics {
		m.Sink.Calc()

		sinkData := m.Sink.Format(data.Time)
		metricsData[name] = sinkData
		if _, ok := m.Sink.(*stats.TrendSink); ok {
			continue
		}

		_, extra := nonTrendMetricValueForSum(data.Time, data.TimeUnit, m)
		if len(extra) > 1 {
			extraData := make(map[string]interface{})
			extraData["value"] = sinkData["value"]
			extraData["extra"] = extra
			metricsData[name] = extraData
		}
	}
	m["metrics"] = metricsData
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")

	return encoder.Encode(m)
}
