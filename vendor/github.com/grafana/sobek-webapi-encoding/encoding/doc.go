// Package encoding implements the WHATWG Encoding Standard for Sobek runtimes.
//
// This package provides TextEncoder and TextDecoder implementations that can be
// registered with a Sobek runtime to provide Web-compatible text encoding/decoding
// capabilities in JavaScript.
//
// # Supported Encodings
//
// The package supports the following encodings:
//   - UTF-8 (default)
//   - UTF-16LE (little-endian)
//   - UTF-16BE (big-endian)
//
// # Usage
//
// RegisterRuntime reads TextDecoder/TextDecoder.decode options (e.g. "fatal",
// "ignoreBOM", "stream") from JS objects by matching their spec names against
// this package's Go struct fields. Sobek only does that matching when a
// [sobek.FieldNameMapper] is configured on the runtime, and
// RegisterRuntime deliberately leaves that choice to the caller: it is a
// runtime-wide setting, and forcing one here would override a field name
// mapper the host application has already configured for its own use (for
// example a JS runtime embedding sobek for other purposes). Callers must set
// one themselves, such as [sobek.TagFieldNameMapper] configured for the "js"
// tag used by this package's option structs, before relying on any TextDecoder
// option. The option structs retain equivalent "json" tags for callers that
// already use a JSON-tag mapper.
//
//	rt := sobek.New()
//	rt.SetFieldNameMapper(sobek.TagFieldNameMapper("js", true))
//	if err := encoding.RegisterRuntime(rt); err != nil {
//	    log.Fatal(err)
//	}
//
// Without a field name mapper, options such as {fatal: true} are silently
// ignored: ExportTo falls back to matching the exact exported Go field name
// (e.g. "Fatal"), which no spec-compliant JS caller will ever pass.
//
// After registration, TextEncoder and TextDecoder are available in JavaScript:
//
//	const encoder = new TextEncoder();
//	const encoded = encoder.encode("Hello, World!");
//
//	const decoder = new TextDecoder("utf-8");
//	const decoded = decoder.decode(encoded);
//
// # Specification
//
// This implementation follows the WHATWG Encoding Standard:
// https://encoding.spec.whatwg.org/
package encoding
