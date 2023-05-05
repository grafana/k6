package fasthttp

import (
	"time"
)

// CoarseTimeNow returns the current time truncated to the nearest second.
//
// Deprecated: This is slower than calling time.Now() directly.
// This is now time.Now().Truncate(time.Second) shortcut.
func CoarseTimeNow() time.Time {
	return time.Now().Truncate(time.Second)
}
