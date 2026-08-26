package icmp

import (
	"time"

	"github.com/grafana/sobek"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func (r *pinger) loop() {
	ctx := r.vu.Context()
	tq := taskqueue.New(r.vu.RegisterCallback)

	defer tq.Close()

	ticker := time.NewTicker(r.opts.interval)
	defer ticker.Stop()

	go r.reader()

	_, err := r.send()
	if err != nil {
		r.callChan <- func() error {
			return r.invokeCallback(nil, err)
		}
	}

	for {
		select {
		case call := <-r.callChan:
			tq.Queue(call)
		case <-ticker.C:
			sent, err := r.send()
			if err != nil {
				r.addErrorMetrics()

				r.callChan <- func() error {
					return r.invokeCallback(nil, err)
				}
			}

			if sent >= r.opts.count {
				ticker.Stop()
			}
		case <-time.After(r.opts.deadline):
			r.log.Warn("Deadline exceeded")

			r.callChan <- func() error {
				r.addErrorMetrics()

				err := r.invokeCallback(nil, errDeadlineExceeded)

				ticker.Stop()

				r.stop <- struct{}{}

				return err
			}

		case <-r.stop:
			return
		case <-ctx.Done():
			r.log.Warn("Context done, stopping pinger loop")

			return
		}
	}
}

func (r *pinger) send() (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sentCount >= r.opts.count {
		return r.sentCount, nil
	}

	seq := (r.opts.seq + r.sentCount) % maxUint16

	msg, err := r.newMessage(seq)
	if err != nil {
		return r.sentCount, err
	}

	if _, err := r.packetConn.WriteTo(msg, r.targetAddr); err != nil {
		r.log.WithError(err).Error("Failed to send ICMP message")

		return r.sentCount, err
	}

	r.log.WithField("sentCount", r.sentCount).Debug("Sent ICMP message")

	r.sentAt[r.sentCount] = time.Now()
	r.seqs[r.sentCount] = seq
	r.sentCount++

	r.addSendMetrics(len(msg))

	return r.sentCount, nil
}

func (r *pinger) newMessage(seq int) ([]byte, error) {
	var msg icmp.Message

	if r.ip6 {
		msg.Type = ipv6.ICMPTypeEchoRequest
	} else {
		msg.Type = ipv4.ICMPTypeEcho
	}

	msg.Body = &icmp.Echo{
		ID:   r.opts.id,
		Seq:  seq,
		Data: r.data,
	}

	return msg.Marshal(nil)
}

func (r *pinger) invokeCallback(detail *pingerDetail, perr error) error {
	if r.opts.callback == nil {
		return nil
	}

	toValue := r.vu.Runtime().ToValue

	pd := toPingDetail(detail, r.vu.Runtime())

	var errValue sobek.Value

	if perr != nil {
		errValue = toValue(wrapError(perr))
	} else {
		errValue = sobek.Null()
	}

	_, err := r.opts.callback(sobek.Undefined(), errValue, toValue(pd))

	return err
}
