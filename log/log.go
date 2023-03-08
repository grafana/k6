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
