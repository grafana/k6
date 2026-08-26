# xk6-tls

k6 extension that fetches and inspects TLS certificates from remote hosts, exposed as a JS promise-based API.

## Architecture

Single-package extension registered as a k6 JavaScript module at init time. The data flow is:

1. A k6 script calls the async API with a target host string.
2. The address is parsed, defaulting to port 443 when no port is given.
3. A goroutine dials the host through k6's VU-level dialer (honoring blocked-hostname rules and context cancellation), performs a TLS handshake, and extracts the first peer certificate.
4. Certificate metadata (subject, issuer, validity dates as Unix millis, SHA-256 fingerprint) is converted to a Sobek value and resolved as a JS promise back on the event loop.

The module follows k6's root-module/per-VU-instance pattern: a singleton root module creates per-VU instances, each holding a reference to the VU for runtime access and network dialing. All network I/O goes through k6's state dialer, never raw net.Dial.

## Gotchas

- TLS verification is intentionally disabled so the extension can inspect expired or self-signed certificates without erroring. This is by design, not a security oversight -- do not "fix" it by enabling verification.
- The API returns dates as Unix milliseconds (not seconds). Changing this breaks every downstream consumer silently because the values are still valid integers, just off by 1000x.
- Address parsing accepts scheme-prefixed URLs (like "https://host") but treats the scheme as a hostname component, causing a confusing "not contain a valid port" error. This is a known rough edge, not a bug to fix in the parser.
- Promise resolution happens on a background goroutine. The resolve/reject calls must only use values created on that goroutine or safely shared -- passing VU-runtime objects created on the main goroutine can cause races.
- CI is delegated to an external shared workflow repository. Workflow changes must be made there, not in this repo.
