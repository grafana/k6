package quic

import (
	"time"

	"github.com/quic-go/quic-go/internal/utils"
)

var deadlineSendImmediately = time.Time{}.Add(42 * time.Millisecond) // any value > time.Time{} and before time.Now() is fine

type connectionTimer struct {
	timer *utils.Timer
	last  time.Time
}

func newTimer() *connectionTimer {
	return &connectionTimer{timer: utils.NewTimer()}
}

func (t *connectionTimer) SetRead() {
	if deadline := t.timer.Deadline(); deadline != deadlineSendImmediately {
		t.last = deadline
	}
	t.timer.SetRead()
}

func (t *connectionTimer) Chan() <-chan time.Time {
	return t.timer.Chan()
}

// SetTimer resets the timer.
// It makes sure that the deadline is strictly increasing.
// This prevents busy-looping in cases where the timer fires, but we can't actually send out a packet.
// This doesn't apply to the pacing deadline, which can be set multiple times to deadlineSendImmediately.
func (t *connectionTimer) SetTimer(idleTimeoutOrKeepAlive, ackAlarm, lossTime, pacing time.Time) {
	deadline := idleTimeoutOrKeepAlive
	if !ackAlarm.IsZero() && ackAlarm.Before(deadline) && ackAlarm.After(t.last) {
		deadline = ackAlarm
	}
	if !lossTime.IsZero() && lossTime.Before(deadline) && lossTime.After(t.last) {
		deadline = lossTime
	}
	if !pacing.IsZero() && pacing.Before(deadline) {
		deadline = pacing
	}
	t.timer.Reset(deadline)
}

func (t *connectionTimer) Stop() {
	t.timer.Stop()
}
