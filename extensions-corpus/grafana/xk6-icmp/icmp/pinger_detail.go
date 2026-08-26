package icmp

import (
	"time"

	"github.com/grafana/sobek"
)

type pingerDetail struct {
	alive           bool
	target          string
	targetIP        string
	targetIPVersion string
	sentAt          time.Time
	receivedAt      time.Time
	ttl             int
	id              int
	seq             int
	size            int
	options         *pingerOptions
}

func newPingerDetail(opts *pingerOptions, target string, targetIP string, targetIPVersion string) *pingerDetail {
	return &pingerDetail{
		ttl:             -1,
		id:              -1,
		seq:             -1,
		size:            -1,
		targetIP:        targetIP,
		targetIPVersion: targetIPVersion,
		target:          target,
		options:         opts,
	}
}

func toPingDetail(src *pingerDetail, rt *sobek.Runtime) *pingDetail {
	if src == nil {
		return nil
	}

	pd := &pingDetail{
		Alive:           src.alive,
		Target:          src.target,
		TargetIp:        src.targetIP,
		TargetIpVersion: src.targetIPVersion,
		SentAt:          sobek.Undefined(),
		ReceivedAt:      sobek.Undefined(),
		Ttl:             sobek.Undefined(),
		Id:              sobek.Undefined(),
		Seq:             sobek.Undefined(),
		Size:            sobek.Undefined(),
		Options:         toPingOptions(src.options, rt),
	}

	if !src.sentAt.IsZero() {
		pd.SentAt = rt.ToValue(src.sentAt.UnixMilli())
	}

	if !src.receivedAt.IsZero() {
		pd.ReceivedAt = rt.ToValue(src.receivedAt.UnixMilli())
	}

	if src.ttl >= 0 {
		pd.Ttl = rt.ToValue(src.ttl)
	}

	if src.id >= 0 {
		pd.Id = rt.ToValue(src.id)
	}

	if src.seq >= 0 {
		pd.Seq = rt.ToValue(src.seq)
	}

	if src.size >= 0 {
		pd.Size = rt.ToValue(src.size)
	}

	return pd
}
