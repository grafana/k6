// Package log implements various logrus hooks.
package log

import (
	"context"

	"github.com/sirupsen/logrus"
)

// AsyncHook extends the logrus.Hook functionality
// handling logging in a not blocking way.
type AsyncHook interface {
	logrus.Hook

	// Listen waits and handles logrus.Hook.Fire events.
	// It stops when the context is canceled.
	Listen(ctx context.Context)
}

// NoticePusher is an optional capability of an AsyncHook: it pushes a single
// diagnostic line immediately, exempt from any per-batch limit the hook
// normally enforces. Callers type-assert an AsyncHook to it to surface events
// (such as an upstream buffer overflow) that must not be lost to that limit.
type NoticePusher interface {
	PushNotice(message string) error
}
