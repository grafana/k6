package metrics

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/guregu/null.v3"
)

// A Metric defines the shape of a set of data.
type Metric struct {
	registry *Registry  `json:"-"`
	Name     string     `json:"name"`
	Type     MetricType `json:"type"`
	Contains ValueType  `json:"contains"`

	// TODO: decouple the metrics from the sinks and thresholds... have them
	// linked, but not in the same struct?
	Tainted    null.Bool    `json:"tainted"`
	Thresholds Thresholds   `json:"thresholds"`
	Submetrics []*Submetric `json:"submetrics"`
	Sub        *Submetric   `json:"-"`
	Sink       Sink         `json:"-"`
	Observed   bool         `json:"-"`
}

// A Submetric represents a filtered dataset based on a parent metric.
type Submetric struct {
	Name   string  `json:"name"`
	Suffix string  `json:"suffix"` // TODO: rename?
	Tags   *TagSet `json:"tags"`

	Metric *Metric `json:"-"`
	Parent *Metric `json:"-"`
}

// AddSubmetric creates a new submetric from the key:value threshold definition
// and adds it to the metric's submetrics list.
func (m *Metric) AddSubmetric(keyValues string) (*Submetric, error) {
	keyValues = strings.TrimSpace(keyValues)
	if len(keyValues) == 0 {
		return nil, fmt.Errorf("submetric criteria for metric '%s' cannot be empty", m.Name)
	}
	kvs := strings.Split(keyValues, ",")
	tags := m.registry.RootTagSet()
	for _, kv := range kvs {
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, ":", 2)

		key := strings.Trim(strings.TrimSpace(parts[0]), `"'`)
		if len(parts) != 2 {
			tags = tags.With(key, "")
			continue
		}

		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		tags = tags.With(key, value)
	}

	for _, sm := range m.Submetrics {
		if tags == sm.Tags {
			return sm, nil
		}
	}

	subMetric := &Submetric{
		Name:   m.Name + "{" + keyValues + "}",
		Suffix: keyValues,
		Tags:   tags,
		Parent: m,
	}
	subMetricMetric := m.registry.newMetric(subMetric.Name, m.Type, m.Contains)
	subMetricMetric.Sub = subMetric // sigh
	subMetric.Metric = subMetricMetric

	m.Submetrics = append(m.Submetrics, subMetric)

	return subMetric, nil
}

// ErrMetricNameParsing indicates parsing a metric name failed
var ErrMetricNameParsing = errors.New("parsing metric name failed")

// ParseMetricName parses a metric name expression of the form metric_name{tag_key:tag_value,...}
// Its first return value is the parsed metric name, second are parsed tags as as slice
// of "key:value" strings. On failure, it returns an error containing the `ErrMetricNameParsing` in its chain.
func ParseMetricName(name string) (string, []string, error) {
	openingTokenPos := strings.IndexByte(name, '{')
	closingTokenPos := strings.LastIndexByte(name, '}')
	containsOpeningToken := openingTokenPos != -1
	containsClosingToken := closingTokenPos != -1

	// Neither the opening '{' token nor the closing '}' token
	// are present, thus the metric name only consists of a literal.
	if !containsOpeningToken && !containsClosingToken {
		return name, nil, nil
	}

	// If the name contains an opening or closing token, but not
	// its counterpart, the expression is malformed.
	if (containsOpeningToken && !containsClosingToken) ||
		(!containsOpeningToken && containsClosingToken) {
		return "", nil, fmt.Errorf(
			"%w, metric %q has unmatched opening/close curly brace",
			ErrMetricNameParsing, name,
		)
	}

	// If the closing brace token appears before the opening one,
	// the expression is malformed
	if closingTokenPos < openingTokenPos {
		return "", nil, fmt.Errorf("%w, metric %q closing curly brace appears before opening one", ErrMetricNameParsing, name)
	}

	// If the last character is not a closing brace token,
	// the expression is malformed.
	if closingTokenPos != (len(name) - 1) {
		err := fmt.Errorf(
			"%w, metric %q lacks a closing curly brace in its last position",
			ErrMetricNameParsing,
			name,
		)
		return "", nil, err
	}

	// We already know the position of the opening and closing curly brace
	// tokens. Thus, we extract the string in between them, and split its
	// content to obtain the tags key values.
	tags := strings.Split(name[openingTokenPos+1:closingTokenPos], ",")

	// For each tag definition, ensure it is correctly formed
	for i, t := range tags {
		keyValue := strings.SplitN(t, ":", 2)

		if len(keyValue) != 2 || keyValue[1] == "" {
			return "", nil, fmt.Errorf("%w, metric %q tag expression is malformed", ErrMetricNameParsing, t)
		}

		tags[i] = strings.TrimSpace(t)
	}

	return name[0:openingTokenPos], tags, nil
}
