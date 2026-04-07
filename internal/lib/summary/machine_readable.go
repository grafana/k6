package summary

import (
	"errors"
	"reflect"
	"time"

	"go.k6.io/k6/internal/build"
	machinereadable "go.k6.io/k6/internal/lib/summary/machinereadable"
	"go.k6.io/k6/internal/lib/summary/machinereadable/cog"
)

var (
	errUnsupportedMetricType     = errors.New("unsupported metric type")
	errUnsupportedMetricContents = errors.New("unsupported metric contents")
)

// ToMachineReadable takes a Summary and Meta and builds a machine-readable summary [machinereadable.Summary] from them.
func ToMachineReadable(s *Summary, meta Meta) (machinereadable.Summary, error) {
	return machinereadable.NewSummaryBuilder().
		Config(machineReadableSummaryConfigBuilder(s, meta)).
		Metadata(machineReadableSummaryMetadataBuilder()).
		Results(machineReadableSummaryResultsBuilder(s)).
		Version(machinereadable.SchemaVersion).
		Build()
}

func machineReadableSummaryConfigBuilder(s *Summary, meta Meta) *machinereadable.SummarySummaryConfigBuilder {
	executionType := machinereadable.SummarySummaryConfigExecutionLocal
	if meta.IsCloud {
		executionType = machinereadable.SummarySummaryConfigExecutionCloud
	}

	return machinereadable.NewSummarySummaryConfigBuilder().
		Duration(s.TestRunDuration.Seconds()).
		Execution(executionType).
		Script(meta.Script)
}

func machineReadableSummaryMetadataBuilder() *machinereadable.SummarySummaryMetadataBuilder {
	return machinereadable.NewSummarySummaryMetadataBuilder().
		K6Version(build.Version).
		GeneratedAt(time.Now())
}

func machineReadableSummaryResultsBuilder(s *Summary) *machinereadable.SummarySummaryResultsBuilder {
	return machinereadable.NewSummarySummaryResultsBuilder().
		Checks(machineReadableSummaryResultsChecksBuilder(s)).
		Metrics(machineReadableSummaryResultsMetricsBuilder(s.Metrics))
}

func machineReadableSummaryResultsChecksBuilder(s *Summary) *machinereadable.SummarySummaryResultsChecksBuilder {
	builder := machinereadable.NewSummarySummaryResultsChecksBuilder()
	if s.Checks == nil {
		return builder
	}

	return builder.
		Metrics([]cog.Builder[machinereadable.Metric]{
			machineReadableMetricBuilder(s.Checks.Metrics.Total),
			machineReadableMetricBuilder(s.Checks.Metrics.Fail),
			machineReadableMetricBuilder(s.Checks.Metrics.Success),
		}).
		Results(machineReadableSummaryResultsChecksResultsBuilder(s.Checks))
}

func machineReadableSummaryResultsMetricsBuilder(metrics Metrics) []cog.Builder[machinereadable.Metric] {
	// We use reflection to traverse the entire struct, so if we ever add a new attribute
	// to the metric's struct, with a new category of metrics, we won't forget to add it here.
	val := reflect.ValueOf(metrics)

	metricBuilders := make([]cog.Builder[machinereadable.Metric], 0)
	for i := 0; i < val.NumField(); i++ {
		value := val.Field(i)
		metricsMap, isMetricsMap := value.Interface().(map[string]Metric)
		if !isMetricsMap {
			continue // Theoretically, this should never happen as all the struct fields satisfy the type.
		}

		for _, m := range metricsMap {
			metricBuilders = append(metricBuilders, machineReadableMetricBuilder(m))
		}
	}
	return metricBuilders
}

func machineReadableSummaryResultsChecksResultsBuilder(
	checks *Checks,
) []cog.Builder[machinereadable.SummarySummaryResultsChecksResults] {
	if checks == nil || len(checks.OrderedChecks) == 0 {
		return nil
	}

	builders := make(
		[]cog.Builder[machinereadable.SummarySummaryResultsChecksResults], 0, len(checks.OrderedChecks),
	)
	for _, c := range checks.OrderedChecks {
		builders = append(builders, machinereadable.NewSummarySummaryResultsChecksResultsBuilder().
			Name(c.Name).
			Passes(c.Passes).
			Fails(c.Fails))
	}
	return builders
}

func machineReadableMetricBuilder(m Metric) *machinereadable.MetricBuilder {
	builder := machinereadable.NewMetricBuilder().Name(m.Name)

	if mType, err := machineReadableMetricType(m.Type); err == nil {
		builder = builder.Type(mType)
	}

	if mContains, err := machineReadableMetricContains(m.Contains); err == nil {
		builder = builder.Contains(mContains)
	}

	if mValues, err := machineReadableMetricValues(m); err == nil {
		builder = builder.Values(mValues)
	}

	return builder
}

func machineReadableMetricType(mType string) (machinereadable.MetricType, error) {
	switch mType {
	case string(machinereadable.MetricTypeCounter):
		return machinereadable.MetricTypeCounter, nil
	case string(machinereadable.MetricTypeGauge):
		return machinereadable.MetricTypeGauge, nil
	case string(machinereadable.MetricTypeRate):
		return machinereadable.MetricTypeRate, nil
	case string(machinereadable.MetricTypeTrend):
		return machinereadable.MetricTypeTrend, nil
	default:
		return "", errUnsupportedMetricType
	}
}

func machineReadableMetricContains(mContains string) (machinereadable.MetricContains, error) {
	switch mContains {
	case string(machinereadable.MetricContainsDefault):
		return machinereadable.MetricContainsDefault, nil
	case string(machinereadable.MetricContainsTime):
		return machinereadable.MetricContainsTime, nil
	case string(machinereadable.MetricContainsData):
		return machinereadable.MetricContainsData, nil
	default:
		return "", errUnsupportedMetricContents
	}
}

func machineReadableMetricValues(m Metric) (any, error) {
	switch m.Type {
	case string(machinereadable.MetricTypeCounter):
		return machinereadable.CounterValues{
			Count: m.Values["count"],
		}, nil
	case string(machinereadable.MetricTypeGauge):
		return machinereadable.GaugeValues{
			Max:   m.Values["max"],
			Min:   m.Values["min"],
			Value: m.Values["value"],
		}, nil
	case string(machinereadable.MetricTypeRate):
		return machinereadable.RateValues{
			Matches: int64(m.Values["passes"]),
			Total:   int64(m.Values["passes"] + m.Values["fails"]),
			Rate:    m.Values["rate"],
		}, nil
	case string(machinereadable.MetricTypeTrend):
		var values machinereadable.TrendValues
		if avgVal, ok := m.Values["avg"]; ok {
			values.Avg = &avgVal
		}
		if maxVal, ok := m.Values["max"]; ok {
			values.Max = &maxVal
		}
		if medVal, ok := m.Values["med"]; ok {
			values.Med = &medVal
		}
		if minVal, ok := m.Values["min"]; ok {
			values.Min = &minVal
		}
		if p90Val, ok := m.Values["p(90)"]; ok {
			values.P90 = &p90Val
		}
		if p95Val, ok := m.Values["p(95)"]; ok {
			values.P95 = &p95Val
		}
		return values, nil
	default:
		return nil, errUnsupportedMetricType
	}
}
