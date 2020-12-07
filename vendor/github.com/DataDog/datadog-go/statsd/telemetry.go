package statsd

import (
	"fmt"
	"sync"
	"time"
)

/*
TelemetryInterval is the interval at which telemetry will be sent by the client.
*/
const TelemetryInterval = 10 * time.Second

/*
clientTelemetryTag is a tag identifying this specific client.
*/
var clientTelemetryTag = "client:go"

/*
clientVersionTelemetryTag is a tag identifying this specific client version.
*/
var clientVersionTelemetryTag = "client_version:4.2.0"

type telemetryClient struct {
	c       *Client
	tags    []string
	sender  *sender
	worker  *worker
	devMode bool
}

func newTelemetryClient(c *Client, transport string, devMode bool) *telemetryClient {
	return &telemetryClient{
		c:       c,
		tags:    append(c.Tags, clientTelemetryTag, clientVersionTelemetryTag, "client_transport:"+transport),
		devMode: devMode,
	}
}

func newTelemetryClientWithCustomAddr(c *Client, transport string, devMode bool, telemetryAddr string, pool *bufferPool) (*telemetryClient, error) {
	telemetryWriter, _, err := resolveAddr(telemetryAddr)
	if err != nil {
		return nil, fmt.Errorf("Could not resolve telemetry address: %v", err)
	}

	t := newTelemetryClient(c, transport, devMode)

	// Creating a custom sender/worker with 1 worker in mutex mode for the
	// telemetry that share the same bufferPool.
	// FIXME due to performance pitfall, we're always using UDP defaults
	// even for UDS.
	t.sender = newSender(telemetryWriter, DefaultUDPBufferPoolSize, pool)
	t.worker = newWorker(pool, t.sender)
	return t, nil
}

func (t *telemetryClient) run(wg *sync.WaitGroup, stop chan struct{}) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(TelemetryInterval)
		for {
			select {
			case <-ticker.C:
				t.sendTelemetry()
			case <-stop:
				ticker.Stop()
				if t.sender != nil {
					t.sender.close()
				}
				return
			}
		}
	}()
}

func (t *telemetryClient) sendTelemetry() {
	for _, m := range t.flush() {
		if t.worker != nil {
			t.worker.processMetric(m)
		} else {
			t.c.send(m)
		}
	}

	if t.worker != nil {
		t.worker.flush()
	}
}

// flushTelemetry returns Telemetry metrics to be flushed. It's its own function to ease testing.
func (t *telemetryClient) flush() []metric {
	m := []metric{}

	// same as Count but without global namespace
	telemetryCount := func(name string, value int64) {
		m = append(m, metric{metricType: count, name: name, ivalue: value, tags: t.tags, rate: 1})
	}

	clientMetrics := t.c.FlushTelemetryMetrics()
	telemetryCount("datadog.dogstatsd.client.metrics", int64(clientMetrics.TotalMetrics))
	if t.devMode {
		telemetryCount("datadog.dogstatsd.client.metricsGauge", int64(clientMetrics.TotalMetricsGauge))
		telemetryCount("datadog.dogstatsd.client.metricsCount", int64(clientMetrics.TotalMetricsCount))
		telemetryCount("datadog.dogstatsd.client.metricsHistogram", int64(clientMetrics.TotalMetricsHistogram))
		telemetryCount("datadog.dogstatsd.client.metricsDistribution", int64(clientMetrics.TotalMetricsDistribution))
		telemetryCount("datadog.dogstatsd.client.metricsSet", int64(clientMetrics.TotalMetricsSet))
		telemetryCount("datadog.dogstatsd.client.metricsTiming", int64(clientMetrics.TotalMetricsTiming))
	}

	telemetryCount("datadog.dogstatsd.client.events", int64(clientMetrics.TotalEvents))
	telemetryCount("datadog.dogstatsd.client.service_checks", int64(clientMetrics.TotalServiceChecks))
	telemetryCount("datadog.dogstatsd.client.metric_dropped_on_receive", int64(clientMetrics.TotalDroppedOnReceive))

	senderMetrics := t.c.sender.flushTelemetryMetrics()
	telemetryCount("datadog.dogstatsd.client.packets_sent", int64(senderMetrics.TotalSentPayloads))
	telemetryCount("datadog.dogstatsd.client.bytes_sent", int64(senderMetrics.TotalSentBytes))
	telemetryCount("datadog.dogstatsd.client.packets_dropped", int64(senderMetrics.TotalDroppedPayloads))
	telemetryCount("datadog.dogstatsd.client.bytes_dropped", int64(senderMetrics.TotalDroppedBytes))
	telemetryCount("datadog.dogstatsd.client.packets_dropped_queue", int64(senderMetrics.TotalDroppedPayloadsQueueFull))
	telemetryCount("datadog.dogstatsd.client.bytes_dropped_queue", int64(senderMetrics.TotalDroppedBytesQueueFull))
	telemetryCount("datadog.dogstatsd.client.packets_dropped_writer", int64(senderMetrics.TotalDroppedPayloadsWriter))
	telemetryCount("datadog.dogstatsd.client.bytes_dropped_writer", int64(senderMetrics.TotalDroppedBytesWriter))

	if aggMetrics := t.c.agg.flushTelemetryMetrics(); aggMetrics != nil {
		telemetryCount("datadog.dogstatsd.client.aggregated_context", int64(aggMetrics.nbContext))
		if t.devMode {
			telemetryCount("datadog.dogstatsd.client.aggregated_context_gauge", int64(aggMetrics.nbContextGauge))
			telemetryCount("datadog.dogstatsd.client.aggregated_context_set", int64(aggMetrics.nbContextSet))
			telemetryCount("datadog.dogstatsd.client.aggregated_context_count", int64(aggMetrics.nbContextCount))
		}
	}

	return m
}
