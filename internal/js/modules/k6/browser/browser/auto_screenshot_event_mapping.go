package browser

import (
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

// mapAutoScreenshotEvent exposes a [common.AutoScreenshotEvent] payload to
// JS scripts that subscribe via `page.on('auto-screenshot', handler)`.
//
// The bytes are exposed as a sobek ArrayBuffer so a handler can hash,
// upload or inspect them without an extra Go↔JS copy. Other fields are
// plain primitives.
func mapAutoScreenshotEvent(vu moduleVU, event common.PageEvent) mapping {
	rt := vu.Runtime()
	ev := event.AutoScreenshot
	if ev == nil {
		return mapping{}
	}

	bytes := rt.NewArrayBuffer(ev.Bytes)

	return mapping{
		"bytes":   bytes,
		"api":     ev.API,
		"reason":  ev.Reason,
		"seq":     ev.Seq,
		"unixMs":  ev.UnixMs,
		"pageUrl": ev.PageURL,
	}
}
