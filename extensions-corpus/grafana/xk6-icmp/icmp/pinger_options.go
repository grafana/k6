package icmp

import (
	"time"

	"github.com/grafana/sobek"
)

const (
	defaultPingTTL       = 64
	defaultPingSize      = 56
	defaultPingInterval  = time.Second
	defaultPingTimeout   = 10 * time.Second
	defaultPingCount     = 1
	defaultPingThreshold = 100.0

	defaultPingMinimumInterval = 500 * time.Millisecond
)

type pingerOptions struct {
	target             string
	callback           sobek.Callable
	id                 int
	seq                int
	ttl                int
	timeout            time.Duration
	deadline           time.Duration
	count              int
	threshold          float64
	size               int
	interval           time.Duration
	source             string
	preferredIPVersion string
	tags               map[string]string
}

func toPingOptions(src *pingerOptions, rt *sobek.Runtime) *pingOptions {
	opts := new(pingOptions)

	toValue := rt.ToValue

	opts.Count = toValue(src.count)
	opts.Deadline = toValue(src.deadline.String())
	opts.Id = toValue(src.id)
	opts.Interval = toValue(src.interval.String())
	opts.PreferredIpVersion = src.preferredIPVersion
	opts.Seq = toValue(src.seq)
	opts.Size = toValue(src.size)
	opts.Source = src.source
	opts.Tags = src.tags
	opts.Threshold = toValue(src.threshold)
	opts.Timeout = toValue(src.timeout.String())
	opts.Ttl = toValue(src.ttl)

	return opts
}

func toPingerOptions(src *pingOptions, target string, callback sobek.Callable) (*pingerOptions, error) {
	opts := &pingerOptions{target: target, callback: callback}
	opts.preferredIPVersion = src.PreferredIpVersion
	opts.source = src.Source
	opts.tags = src.Tags

	var err error

	opts.size, err = toInt(src.Size, defaultPingSize)
	if err != nil {
		return nil, err
	}

	opts.count, err = toInt(src.Count, defaultPingCount)
	if err != nil {
		return nil, err
	}

	opts.ttl, err = toInt(src.Ttl, defaultPingTTL)
	if err != nil {
		return nil, err
	}

	if dur, err := toDuration(src.Timeout, defaultPingTimeout); err == nil {
		opts.timeout = dur
	} else {
		return nil, err
	}

	if dur, err := toDuration(src.Interval, defaultPingInterval); err == nil {
		opts.interval = dur
	} else {
		return nil, err
	}

	defaultDeadline := time.Duration(opts.count-1)*opts.interval + opts.timeout + opts.timeout>>1

	if dur, err := toDuration(src.Deadline, defaultDeadline); err == nil {
		opts.deadline = dur
	} else {
		return nil, err
	}

	opts.id, err = toUint16(src.Id)
	if err != nil {
		return nil, err
	}

	opts.seq, err = toUint16(src.Seq)
	if err != nil {
		return nil, err
	}

	pc, err := toPercent(src.Threshold, defaultPingThreshold)
	if err != nil {
		return nil, err
	}

	opts.threshold = pc

	return opts, nil
}
