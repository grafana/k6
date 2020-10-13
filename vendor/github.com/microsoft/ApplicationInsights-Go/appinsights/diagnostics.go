package appinsights

import (
	"fmt"
	"sync"
)

type diagnosticsMessageWriter struct {
	listeners []*diagnosticsMessageListener
	lock      sync.Mutex
}

// Handler function for receiving diagnostics messages.  If this returns an
// error, then the listener will be removed.
type DiagnosticsMessageHandler func(string) error

// Listener type returned by NewDiagnosticsMessageListener.
type DiagnosticsMessageListener interface {
	// Stop receiving diagnostics messages from this listener.
	Remove()
}

type diagnosticsMessageListener struct {
	handler DiagnosticsMessageHandler
	writer  *diagnosticsMessageWriter
}

func (listener *diagnosticsMessageListener) Remove() {
	listener.writer.removeListener(listener)
}

// The one and only diagnostics writer.
var diagnosticsWriter = &diagnosticsMessageWriter{}

// Subscribes the specified handler to diagnostics messages from the SDK.  The
// returned interface can be used to unsubscribe.
func NewDiagnosticsMessageListener(handler DiagnosticsMessageHandler) DiagnosticsMessageListener {
	listener := &diagnosticsMessageListener{
		handler: handler,
		writer:  diagnosticsWriter,
	}

	diagnosticsWriter.appendListener(listener)
	return listener
}

func (writer *diagnosticsMessageWriter) appendListener(listener *diagnosticsMessageListener) {
	writer.lock.Lock()
	defer writer.lock.Unlock()
	writer.listeners = append(writer.listeners, listener)
}

func (writer *diagnosticsMessageWriter) removeListener(listener *diagnosticsMessageListener) {
	writer.lock.Lock()
	defer writer.lock.Unlock()

	for i := 0; i < len(writer.listeners); i++ {
		if writer.listeners[i] == listener {
			writer.listeners[i] = writer.listeners[len(writer.listeners)-1]
			writer.listeners = writer.listeners[:len(writer.listeners)-1]
			return
		}
	}
}

func (writer *diagnosticsMessageWriter) Write(message string) {
	var toRemove []*diagnosticsMessageListener
	for _, listener := range writer.listeners {
		if err := listener.handler(message); err != nil {
			toRemove = append(toRemove, listener)
		}
	}

	for _, listener := range toRemove {
		listener.Remove()
	}
}

func (writer *diagnosticsMessageWriter) Printf(message string, args ...interface{}) {
	// Don't bother with Sprintf if nobody is listening
	if writer.hasListeners() {
		writer.Write(fmt.Sprintf(message, args...))
	}
}

func (writer *diagnosticsMessageWriter) hasListeners() bool {
	return len(writer.listeners) > 0
}
