//go:build go1.21

package qtls

import (
	"crypto/tls"
)

type clientSessionCache struct {
	getData func() []byte
	setData func([]byte) (allowEarlyData bool)
	wrapped tls.ClientSessionCache
}

var _ tls.ClientSessionCache = &clientSessionCache{}

func (c clientSessionCache) Put(key string, cs *tls.ClientSessionState) {
	if cs == nil {
		c.wrapped.Put(key, nil)
		return
	}
	ticket, state, err := cs.ResumptionState()
	if err != nil || state == nil {
		c.wrapped.Put(key, cs)
		return
	}
	state.Extra = append(state.Extra, addExtraPrefix(c.getData()))
	newCS, err := tls.NewResumptionState(ticket, state)
	if err != nil {
		// It's not clear why this would error. Just save the original state.
		c.wrapped.Put(key, cs)
		return
	}
	c.wrapped.Put(key, newCS)
}

func (c clientSessionCache) Get(key string) (*tls.ClientSessionState, bool) {
	cs, ok := c.wrapped.Get(key)
	if !ok || cs == nil {
		return cs, ok
	}
	ticket, state, err := cs.ResumptionState()
	if err != nil {
		// It's not clear why this would error.
		// Remove the ticket from the session cache, so we don't run into this error over and over again
		c.wrapped.Put(key, nil)
		return nil, false
	}
	var earlyData bool
	// restore QUIC transport parameters and RTT stored in state.Extra
	if extra := findExtraData(state.Extra); extra != nil {
		earlyData = c.setData(extra)
	}
	state.EarlyData = earlyData
	session, err := tls.NewResumptionState(ticket, state)
	if err != nil {
		// It's not clear why this would error.
		// Remove the ticket from the session cache, so we don't run into this error over and over again
		c.wrapped.Put(key, nil)
		return nil, false
	}
	return session, true
}
