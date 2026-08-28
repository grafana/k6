package icmp

import (
	"time"

	extensionapi "go.k6.io/k6-extension-api"
)

func (r *pinger) currentTags() extensionapi.Tags { return r.metrics.host.CurrentTags() }

func (r *pinger) tags() extensionapi.Tags {
	tags := r.currentTags().With(map[string]string{"proto": "ICMP", "ip": r.targetIP.String()})
	return tags.With(r.opts.tags)
}

func (r *pinger) emit(samples []extensionapi.Sample) {
	_ = r.metrics.host.Emit(r.vu.Context(), samples)
}

func (r *pinger) addErrorMetrics() {
	r.emit([]extensionapi.Sample{{Metric: r.metrics.icmpErrors, Value: 1, Tags: r.tags()}})
}

func (r *pinger) addSendMetrics(size int) {
	r.emit([]extensionapi.Sample{
		{Metric: r.metrics.icmpPacketsSent, Value: 1, Tags: r.tags()},
		{Metric: r.metrics.dataSent, Value: float64(size), Tags: r.currentTags()},
	})
}

func (r *pinger) addReceivedMetrics(size, ttl int, sentAt time.Time) {
	rtt := time.Since(sentAt)
	samples := []extensionapi.Sample{
		{Metric: r.metrics.icmpPacketsReceived, Value: 1, Tags: r.tags()},
		{Metric: r.metrics.dataReceived, Value: float64(size), Tags: r.currentTags()},
		{Metric: r.metrics.icmpRtt, Value: float64(rtt.Milliseconds()), Tags: r.tags()},
	}
	if ttl > 0 {
		samples = append(samples, extensionapi.Sample{Metric: r.metrics.icmpReplyTTL, Value: float64(ttl), Tags: r.currentTags()})
	}
	r.emit(samples)
}

func (r *pinger) addDurationMetrics(startedAt time.Time, metric extensionapi.Metric) {
	r.emit([]extensionapi.Sample{{Metric: metric, Value: float64(time.Since(startedAt).Milliseconds()), Tags: r.tags()}})
}
