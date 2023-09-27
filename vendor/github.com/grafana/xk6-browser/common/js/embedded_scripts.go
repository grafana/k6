package js

import (
	_ "embed"
)

// WebVitalIIFEScript was downloaded from
// https://unpkg.com/web-vitals@3/dist/web-vitals.iife.js.
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

// K6ObjectScript is used to propagate
// information to other libraries about
// the current user session.
//
//go:embed k6_object.js
var K6ObjectScript string
