package runtime

import (
	"os"
	"os/signal"
)

// Signals define methods for handling signals
type Signals interface {
	// Notify returns a channel for receiving notifications of the given signals
	Notify(...os.Signal) <-chan os.Signal
	// Reset stops receiving signal notifications in this channel.
	// If no signal is specified, all signals are cleared
	Reset(...os.Signal)
}

// implements the Signals interface
type signals struct {
	channel chan os.Signal
}

// DefaultSignals returns a default signal handler
func DefaultSignals() Signals {
	return &signals{
		channel: make(chan os.Signal, 1),
	}
}

// Notify implements Signal interface's Notify method
func (s *signals) Notify(signals ...os.Signal) <-chan os.Signal {
	signal.Notify(s.channel, signals...)

	return s.channel
}

// Reset implements Signal interface's Reset method
func (s *signals) Reset(signals ...os.Signal) {
	signal.Reset(signals...)
}
