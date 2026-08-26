package icmp

import (
	"time"

	"go.k6.io/k6/v2/metrics"
)

func (r *pinger) currentTags() *metrics.TagSet {
	return r.vu.State().Tags.GetCurrentValues().Tags
}

func (r *pinger) tags() *metrics.TagSet {
	tags := r.currentTags().With("proto", "ICMP").With("ip", r.targetIP.String())

	return addToTagSet(tags, r.opts.tags)
}

func (r *pinger) addErrorMetrics() {
	metrics.PushIfNotDone(r.vu.Context(), r.vu.State().Samples, metrics.Samples{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: r.metrics.icmpErrors,
				Tags:   r.tags(),
			},
			Time:  time.Now(),
			Value: float64(1),
		},
	})
}

func (r *pinger) addSendMetrics(size int) {
	now := time.Now()

	samples := metrics.Samples{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: r.metrics.icmpPacketsSent,
				Tags:   r.tags(),
			},
			Time:  now,
			Value: float64(1),
		},
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: r.metrics.dataSent,
				Tags:   r.currentTags(),
			},
			Time:  now,
			Value: float64(size),
		},
	}

	metrics.PushIfNotDone(r.vu.Context(), r.vu.State().Samples, samples)
}

func (r *pinger) addReceivedMetrics(size int, ttl int, sentAt time.Time) {
	now := time.Now()
	rtt := now.Sub(sentAt)

	samples := metrics.Samples{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: r.metrics.icmpPacketsReceived,
				Tags:   r.tags(),
			},
			Time:  now,
			Value: float64(1),
		},
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: r.metrics.dataReceived,
				Tags:   r.currentTags(),
			},
			Time:  now,
			Value: float64(size),
		},
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: r.metrics.icmpRtt,
				Tags:   r.tags(),
			},
			Time:  now,
			Value: float64(rtt.Milliseconds()),
		},
	}

	if ttl > 0 {
		samples = append(samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: r.metrics.icmpReplyTTL,
				Tags:   r.currentTags(),
			},
			Time:  now,
			Value: float64(ttl),
		})
	}

	metrics.PushIfNotDone(r.vu.Context(), r.vu.State().Samples, samples)
}

func (r *pinger) addDurationMetrics(startedAt time.Time, method *metrics.Metric) {
	now := time.Now()

	samples := metrics.Samples{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: method,
				Tags:   r.tags(),
			},
			Time:  now,
			Value: float64(now.Sub(startedAt).Milliseconds()),
		},
	}

	metrics.PushIfNotDone(r.vu.Context(), r.vu.State().Samples, samples)
}
