package js

import (
	_ "embed"
)

// WebVitalIIFEScript was downloaded from
// https://unpkg.com/web-vitals@5.1.0/dist/web-vitals.iife.js.
// Repo: https://github.com/GoogleChrome/web-vitals
//
//go:embed web_vital_iife.js
var WebVitalIIFEScript string

// WebVitalInitScript uses WebVitalIIFEScript
// and applies it to the current website that
// this init script is used against.
//
//go:embed web_vital_init.js
var WebVitalInitScript string

// DOMMutationObserverScript installs a MutationObserver that signals the
// k6browserDomMutation CDP binding whenever the document changes. Used by
// the auto-screenshot lifecycle watcher in Mode B to detect SPA state
// transitions that do not fire lifecycle events.
//
//go:embed dom_mutation_observer.js
var DOMMutationObserverScript string
