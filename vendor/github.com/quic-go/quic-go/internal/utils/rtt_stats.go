package utils

import (
	"time"

	"github.com/quic-go/quic-go/internal/protocol"
)

const (
	rttAlpha      = 0.125
	oneMinusAlpha = 1 - rttAlpha
	rttBeta       = 0.25
	oneMinusBeta  = 1 - rttBeta
	// The default RTT used before an RTT sample is taken.
	defaultInitialRTT = 100 * time.Millisecond
)

// RTTStats provides round-trip statistics
type RTTStats struct {
	hasMeasurement bool

	minRTT        time.Duration
	latestRTT     time.Duration
	smoothedRTT   time.Duration
	meanDeviation time.Duration

	maxAckDelay time.Duration
}

// NewRTTStats makes a properly initialized RTTStats object
func NewRTTStats() *RTTStats {
	return &RTTStats{}
}

// MinRTT Returns the minRTT for the entire connection.
// May return Zero if no valid updates have occurred.
func (r *RTTStats) MinRTT() time.Duration { return r.minRTT }

// LatestRTT returns the most recent rtt measurement.
// May return Zero if no valid updates have occurred.
func (r *RTTStats) LatestRTT() time.Duration { return r.latestRTT }

// SmoothedRTT returns the smoothed RTT for the connection.
// May return Zero if no valid updates have occurred.
func (r *RTTStats) SmoothedRTT() time.Duration { return r.smoothedRTT }

// MeanDeviation gets the mean deviation
func (r *RTTStats) MeanDeviation() time.Duration { return r.meanDeviation }

// MaxAckDelay gets the max_ack_delay advertised by the peer
func (r *RTTStats) MaxAckDelay() time.Duration { return r.maxAckDelay }

// PTO gets the probe timeout duration.
func (r *RTTStats) PTO(includeMaxAckDelay bool) time.Duration {
	if r.SmoothedRTT() == 0 {
		return 2 * defaultInitialRTT
	}
	pto := r.SmoothedRTT() + Max(4*r.MeanDeviation(), protocol.TimerGranularity)
	if includeMaxAckDelay {
		pto += r.MaxAckDelay()
	}
	return pto
}

// UpdateRTT updates the RTT based on a new sample.
func (r *RTTStats) UpdateRTT(sendDelta, ackDelay time.Duration, now time.Time) {
	if sendDelta == InfDuration || sendDelta <= 0 {
		return
	}

	// Update r.minRTT first. r.minRTT does not use an rttSample corrected for
	// ackDelay but the raw observed sendDelta, since poor clock granularity at
	// the client may cause a high ackDelay to result in underestimation of the
	// r.minRTT.
	if r.minRTT == 0 || r.minRTT > sendDelta {
		r.minRTT = sendDelta
	}

	// Correct for ackDelay if information received from the peer results in a
	// an RTT sample at least as large as minRTT. Otherwise, only use the
	// sendDelta.
	sample := sendDelta
	if sample-r.minRTT >= ackDelay {
		sample -= ackDelay
	}
	r.latestRTT = sample
	// First time call.
	if !r.hasMeasurement {
		r.hasMeasurement = true
		r.smoothedRTT = sample
		r.meanDeviation = sample / 2
	} else {
		r.meanDeviation = time.Duration(oneMinusBeta*float32(r.meanDeviation/time.Microsecond)+rttBeta*float32(AbsDuration(r.smoothedRTT-sample)/time.Microsecond)) * time.Microsecond
		r.smoothedRTT = time.Duration((float32(r.smoothedRTT/time.Microsecond)*oneMinusAlpha)+(float32(sample/time.Microsecond)*rttAlpha)) * time.Microsecond
	}
}

// SetMaxAckDelay sets the max_ack_delay
func (r *RTTStats) SetMaxAckDelay(mad time.Duration) {
	r.maxAckDelay = mad
}

// SetInitialRTT sets the initial RTT.
// It is used during the 0-RTT handshake when restoring the RTT stats from the session state.
func (r *RTTStats) SetInitialRTT(t time.Duration) {
	// On the server side, by the time we get to process the session ticket,
	// we might already have obtained an RTT measurement.
	// This can happen if we received the ClientHello in multiple pieces, and one of those pieces was lost.
	// Discard the restored value. A fresh measurement is always better.
	if r.hasMeasurement {
		return
	}
	r.smoothedRTT = t
	r.latestRTT = t
}

// OnConnectionMigration is called when connection migrates and rtt measurement needs to be reset.
func (r *RTTStats) OnConnectionMigration() {
	r.latestRTT = 0
	r.minRTT = 0
	r.smoothedRTT = 0
	r.meanDeviation = 0
}

// ExpireSmoothedMetrics causes the smoothed_rtt to be increased to the latest_rtt if the latest_rtt
// is larger. The mean deviation is increased to the most recent deviation if
// it's larger.
func (r *RTTStats) ExpireSmoothedMetrics() {
	r.meanDeviation = Max(r.meanDeviation, AbsDuration(r.smoothedRTT-r.latestRTT))
	r.smoothedRTT = Max(r.smoothedRTT, r.latestRTT)
}
