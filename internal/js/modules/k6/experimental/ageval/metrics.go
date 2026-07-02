package ageval

import (
	"time"

	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/metrics"
)

// agevalMetrics holds the custom metrics emitted by the module. They are
// standard k6 metric types (Trend/Rate/Counter) so existing k6 outputs and
// Grafana Cloud dashboards render them with no extra configuration.
type agevalMetrics struct {
	duration        *metrics.Metric // Trend (time): wall-clock of a full agent run
	steps           *metrics.Metric // Trend: number of model round-trips per run
	toolCalls       *metrics.Metric // Counter: tool calls, tagged by tool name
	tokens          *metrics.Metric // Counter: tokens, tagged by direction
	cost            *metrics.Metric // Counter: estimated USD spend
	toolCorrectness *metrics.Metric // Rate: expectSequence pass/fail
	qualityScore    *metrics.Metric // Trend: LLM-as-judge score 0..1
	judgePass       *metrics.Metric // Rate: judge score >= threshold
	judgeTokens     *metrics.Metric // Counter: judge model tokens, tagged by direction
	judgeCost       *metrics.Metric // Counter: estimated USD spend by the judge itself
}

// registerMetrics creates (or reuses) the module metrics in the registry.
func registerMetrics(registry *metrics.Registry) *agevalMetrics {
	return &agevalMetrics{
		duration:        registry.MustNewMetric("agent_duration", metrics.Trend, metrics.Time),
		steps:           registry.MustNewMetric("agent_steps", metrics.Trend),
		toolCalls:       registry.MustNewMetric("agent_tool_calls", metrics.Counter),
		tokens:          registry.MustNewMetric("agent_tokens", metrics.Counter),
		cost:            registry.MustNewMetric("agent_cost_usd", metrics.Counter),
		toolCorrectness: registry.MustNewMetric("agent_tool_correctness", metrics.Rate),
		qualityScore:    registry.MustNewMetric("agent_quality_score", metrics.Trend),
		judgePass:       registry.MustNewMetric("agent_judge_pass", metrics.Rate),
		judgeTokens:     registry.MustNewMetric("agent_judge_tokens", metrics.Counter),
		judgeCost:       registry.MustNewMetric("agent_judge_cost_usd", metrics.Counter),
	}
}

// pushSample emits a single metric sample with the given tags. It is a no-op in
// the init context (no VU state), mirroring how check() behaves.
func pushSample(vu modules.VU, m *metrics.Metric, tags *metrics.TagSet, value float64) {
	state := vu.State()
	if state == nil {
		return
	}
	metrics.PushIfNotDone(vu.Context(), state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{Metric: m, Tags: tags},
		Time:       time.Now(),
		Value:      value,
	})
}
