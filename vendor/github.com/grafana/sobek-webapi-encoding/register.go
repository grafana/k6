// Package sobekencoding registers the encoding Web API with Sobek runtimes.
package sobekencoding

import (
	"github.com/grafana/sobek"

	"github.com/grafana/sobek-webapi-encoding/encoding"
)

// RegisterGlobally exposes the encoding TextDecoder/TextEncoder constructors in the provided sobek runtime.
//
// See [encoding.RegisterRuntime] for a required caveat about rt's field name
// mapper: without one configured to resolve the option structs' "js" or
// "json" tags, TextDecoder options such as "fatal" are silently ignored.
func RegisterGlobally(rt *sobek.Runtime) error {
	return encoding.RegisterRuntime(rt)
}
