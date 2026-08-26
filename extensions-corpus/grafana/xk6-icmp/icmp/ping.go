package icmp

import (
	"fmt"
	"reflect"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/promises"
)

type pingOptions struct {
	Id                 sobek.Value //nolint:revive
	Seq                sobek.Value
	Ttl                sobek.Value //nolint:revive
	Size               sobek.Value
	Timeout            sobek.Value
	Deadline           sobek.Value
	Count              sobek.Value
	Threshold          sobek.Value
	Interval           sobek.Value
	Source             string
	PreferredIpVersion string //nolint:revive
	Tags               map[string]string
}

type pingDetail struct {
	Alive           bool
	Target          string
	TargetIp        string //nolint:revive
	TargetIpVersion string //nolint:revive
	SentAt          sobek.Value
	ReceivedAt      sobek.Value
	Id              sobek.Value //nolint:revive
	Seq             sobek.Value
	Ttl             sobek.Value //nolint:revive
	Size            sobek.Value
	Options         *pingOptions
}

func (m *module) ping(target string, options sobek.Value) (bool, error) {
	opts, err := m.pingPrepare(target, options, nil)
	if err != nil {
		return false, err
	}

	return m.pingExecute(opts)
}

func (m *module) pingAsync(target string, optsOrCb sobek.Value, cbOrNil sobek.Callable) (*sobek.Promise, error) {
	opts, err := m.pingPrepare(target, optsOrCb, cbOrNil)
	if err != nil {
		return nil, err
	}

	promise, resolve, reject := promises.New(m.vu)

	go func() {
		result, err := m.pingExecute(opts)
		if err != nil {
			reject(err)

			return
		}

		resolve(m.vu.Runtime().ToValue(result))
	}()

	return promise, nil
}

func (m *module) pingPrepare(target string, optsOrCb sobek.Value, cbOrNil sobek.Callable) (*pingerOptions, error) {
	if sobek.IsUndefined(optsOrCb) || sobek.IsNull(optsOrCb) || optsOrCb == nil {
		return toPingerOptions(new(pingOptions), target, nil)
	}

	var (
		opts *pingOptions
		err  error
		fn   sobek.Callable
	)

	switch optsOrCb.ExportType() {
	case reflect.TypeFor[map[string]any]():
		opts = new(pingOptions)
		fn = cbOrNil
		err = m.vu.Runtime().ExportTo(optsOrCb, opts)
	default:
		if callable, ok := sobek.AssertFunction(optsOrCb); ok {
			fn = callable
			opts = new(pingOptions)
		} else {
			err = fmt.Errorf("%w: String or ArrayBuffer expected", errInvalidType)
		}
	}

	if err != nil {
		return nil, err
	}

	popts, err := toPingerOptions(opts, target, fn)
	if err != nil {
		return nil, err
	}

	if minval := m.getIntervalMin(); popts.interval < minval {
		m.log.Warnf("specified interval %s is less than allowed minimum %s, using minimum", popts.interval, minval)
		popts.interval = minval
	}

	return popts, nil
}

func (m *module) pingExecute(opts *pingerOptions) (bool, error) {
	return newPinger(opts, m.vu, m.log, m.metrics).run()
}

func (m *module) getIntervalMin() time.Duration {
	env, ok := m.lookupEnv("K6_PING_MINIMUM_INTERVAL")
	if !ok {
		return defaultPingMinimumInterval
	}

	dur, err := time.ParseDuration(env)
	if err != nil {
		m.log.Warnf("invalid K6_PING_MINIMUM_INTERVAL value: %s, using default %s", err, defaultPingMinimumInterval)

		return defaultPingMinimumInterval
	}

	return dur
}
