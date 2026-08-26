package kafka

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

const (
	GoErrorPrefix = "GoError: "
)

// struct to keep all the things test need in one place.
type kafkaTest struct {
	tb            testing.TB
	topicName     string
	rt            *sobek.Runtime
	module        *Module
	vu            *modulestest.VU
	samples       chan metrics.SampleContainer
	cancelContext context.CancelFunc
}

// getTestModuleInstance returns a new instance of the Kafka module for testing.
// nolint: revive
func getTestModuleInstance(tb testing.TB) *kafkaTest {
	tb.Helper()
	runtime := sobek.New()
	runtime.SetFieldNameMapper(common.FieldNameMapper{})

	ctx, cancel := context.WithCancel(context.Background())
	tb.Cleanup(cancel)

	root := New()
	registry := metrics.NewRegistry()
	mockVU := &modulestest.VU{
		RuntimeField: runtime,
		InitEnvField: &common.InitEnvironment{
			TestPreInitState: &lib.TestPreInitState{
				Registry: registry,
			},
		},
		CtxField: ctx,
	}
	moduleInstance, ok := root.NewModuleInstance(mockVU).(*Module)
	require.True(tb, ok)

	require.NoError(tb, runtime.Set("kafka", moduleInstance.Exports().Default))
	topicName := fmt.Sprintf("%s-%d", tb.Name(), time.Now().UnixMilli())

	return &kafkaTest{
		tb:            tb,
		topicName:     topicName,
		rt:            runtime,
		module:        moduleInstance,
		vu:            mockVU,
		cancelContext: cancel,
	}
}

// moveToVUCode moves to the VU code from the init code (to test certain functions).
func (k *kafkaTest) moveToVUCode() {
	samples := make(chan metrics.SampleContainer, 1000)
	// Save it, so we can reuse it in other tests
	k.samples = samples

	registry := metrics.NewRegistry()

	state := &lib.State{
		Options: lib.Options{
			UserAgent: null.StringFrom("TestUserAgent"),
			Paused:    null.BoolFrom(false),
		},
		BufferPool:     lib.NewBufferPool(),
		Samples:        k.samples,
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
	}
	k.vu.StateField = state
	k.vu.InitEnvField = nil
}

// getCounterMetricsValues returns the samples of the collected metrics in the VU.
func (k *kafkaTest) getCounterMetricsValues() map[string]float64 {
	metricsValues := make(map[string]float64)

	for _, sampleContainer := range metrics.GetBufferedSamples(k.samples) {
		for _, sample := range sampleContainer.GetSamples() {
			if sample.Metric.Type == metrics.Counter {
				metricsValues[sample.Metric.Name] = sample.Value
			}
		}
	}
	return metricsValues
}

func (k *kafkaTest) getMetricValues() map[string]float64 {
	metricsValues := make(map[string]float64)

	for _, sampleContainer := range metrics.GetBufferedSamples(k.samples) {
		for _, sample := range sampleContainer.GetSamples() {
			metricsValues[sample.Metric.Name] = sample.Value
		}
	}

	return metricsValues
}

// newWriter creates a Kafka writer for the reader tests.
func (k *kafkaTest) newWriter() *Producer {
	producer, err := NewProducerFromWriterConfig(&WriterConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   k.topicName,
	})
	require.NoError(k.t(), err)
	return producer
}

// newReader creates a Kafka reader for the reader tests.
func (k *kafkaTest) newReader() *Consumer {
	consumer, err := NewConsumerFromReaderConfig(&ReaderConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   k.topicName,
	})
	require.NoError(k.t(), err)
	return consumer
}

func (k *kafkaTest) newAdminClient() *AdminClient {
	adminClient, err := NewAdminClientFromConnectionConfig(&ConnectionConfig{
		Address: "localhost:9092",
	})
	require.NoError(k.t(), err)
	return adminClient
}

func (k *kafkaTest) t() testing.TB {
	return k.tb
}

// createTopic creates a topic.
func (k *kafkaTest) createTopic() {
	adminClient := k.newAdminClient()
	defer func() {
		_ = adminClient.Close()
	}()

	require.NoError(k.t(), adminClient.CreateTopic(context.Background(), TopicConfig{Topic: k.topicName}))
}

// topicExists checks if a topic exists.
func (k *kafkaTest) topicExists() bool {
	adminClient := k.newAdminClient()
	defer func() {
		_ = adminClient.Close()
	}()

	topics, err := adminClient.ListTopics(context.Background())
	require.NoError(k.t(), err)

	return slices.ContainsFunc(topics, func(topic TopicInfo) bool {
		return topic.Topic == k.topicName
	})
}
