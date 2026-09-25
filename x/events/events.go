// Package events contains types necessary to interact with the event system
// used to notify external components of various internal events during test
// execution.
//
// Experimental: This package is experimental and may be changed, renamed or
// removed in a later k6 release.
package events

import "go.k6.io/k6/v2/internal/event"

// Type represents the different event types emitted by k6.
//
// The constants of this type can be passed to the Subscribe method available
// on the Subscriber interfaces available by calling the Event() method of the
// go.k6.io/k6/v2/js/modules.VU interface.
//
// Look at go.k6.io/k6/v2/internal/event.Subscriber for details.
type Type = event.Type

const (
	// Init is emitted when k6 starts initializing outputs, VUs and executors.
	Init Type = event.Init
	// TestStart is emitted when the execution scheduler starts running the test.
	TestStart = event.TestStart
	// TestEnd is emitted when the test execution ends.
	TestEnd = event.TestEnd
	// IterStart is emitted when a VU starts an iteration.
	IterStart = event.IterStart
	// IterEnd is emitted when a VU ends an iteration.
	IterEnd = event.IterEnd
	// Exit is emitted when the k6 process is about to exit.
	Exit = event.Exit
)

// Event is the emitted object sent to all subscribers of its type.
// The subscriber should call its Done method when finished processing
// to notify the emitter, though this is not required for all events.
//
// This is the type sent over the channel returned by calling the Subscribe
// method of the go.k6.io/k6/v2/internal/event.Subscriber interface.
type Event = event.Event

// ExitData is the data sent in the Exit event. Error is the error returned by
// the run command.
type ExitData = event.ExitData

// IterData is the data sent in the IterStart and IterEnd events.
type IterData = event.IterData
