package lib

import (
	"io"
	"strings"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/internal/event"
	"go.k6.io/k6/internal/lib/trace"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/metrics"
)

// TestPreInitState contains all of the state that can be gathered and built
// before the test run is initialized.
type TestPreInitState struct {
	RuntimeOptions RuntimeOptions
	Registry       *metrics.Registry
	BuiltinMetrics *metrics.BuiltinMetrics
	Events         *event.System
	KeyLogger      io.Writer
	LookupEnv      func(key string) (val string, ok bool)
	Logger         logrus.FieldLogger
	TracerProvider *trace.TracerProvider
	Usage          *usage.Usage
}

// TestRunState contains the pre-init state as well as all of the state and
// options that are necessary for actually running the test.
type TestRunState struct {
	*TestPreInitState

	Options Options
	Runner  Runner // TODO: rename to something better, see type comment
	RunTags *metrics.TagSet

	GroupSummary *GroupSummary // TODO(@mstoykov): move and rename

	// TODO: add other properties that are computed or derived after init, e.g.
	// thresholds?
}

// GroupSummaryDescription is the description of the GroupSummary used to identify and ignore it
// for the purposes of the cli descriptions.
const GroupSummaryDescription = "Internal Group Summary output"

// GroupSummary is an internal output implementation that facilitates the aggregation of
// group and check metrics for the purposes of the end of test summary and the REST API
type GroupSummary struct {
	group   *Group // TODO(@mstoykov): move the whole type outside of lib later
	ch      chan []metrics.SampleContainer
	stopped chan struct{}
	logger  logrus.FieldLogger
}

// NewGroupSummary returns new GroupSummary ready to be started.
func NewGroupSummary(logger logrus.FieldLogger) *GroupSummary {
	group, _ := NewGroup(RootGroupPath, nil)
	return &GroupSummary{
		group:   group,
		ch:      make(chan []metrics.SampleContainer, 1000),
		stopped: make(chan struct{}),
		logger:  logger,
	}
}

// Group returns the underlying group that has been aggregated
func (gs *GroupSummary) Group() *Group {
	return gs.group
}

// Description is part of the output.Output interface
func (gs *GroupSummary) Description() string {
	return GroupSummaryDescription
}

func buildGroup(rootGroup *Group, groupName string) (*Group, error) {
	group := rootGroup
	groups := strings.Split(groupName, "::")
	var err error
	for _, groupName := range groups[1:] {
		group, err = group.Group(groupName)
		if err != nil {
			return nil, err
		}
	}
	return group, nil
}

func (gs *GroupSummary) handleSample(sample metrics.Sample) error {
	switch sample.Metric.Name {
	case "group_duration", "checks":
		groupName, ok := sample.Tags.Get(metrics.TagGroup.String())
		if !ok {
			return nil
		}
		group, err := buildGroup(gs.group, groupName)
		if err != nil {
			return err
		}

		if sample.Metric.Name != "checks" {
			return nil
		}

		checkName, ok := sample.Tags.Get(metrics.TagCheck.String())
		if !ok {
			return nil
		}
		check, err := group.Check(checkName)
		if err != nil {
			return err
		}
		if sample.Value == 0 {
			atomic.AddInt64(&check.Fails, 1)
		} else {
			atomic.AddInt64(&check.Passes, 1)
		}
	}
	return nil
}

// Start is part of the output.Output interface
func (gs *GroupSummary) Start() error {
	go func() {
		defer close(gs.stopped)
		for containers := range gs.ch {
			for _, container := range containers {
				for _, sample := range container.GetSamples() {
					err := gs.handleSample(sample)
					if err != nil {
						gs.logger.WithError(err).Warn("couldn't handle a sample as part of group summary")
					}
				}
			}
		}
	}()
	return nil
}

// Stop is part of the output.Output interface
func (gs *GroupSummary) Stop() error {
	close(gs.ch)
	<-gs.stopped
	return nil
}

// AddMetricSamples is part of the output.Output interface
func (gs *GroupSummary) AddMetricSamples(samples []metrics.SampleContainer) {
	gs.ch <- samples
}
