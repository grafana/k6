package dns

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/grafana/sobek"
	miekgdns "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

const (
	// testDomain is the domain name we configure our test DNS server to resolve to the
	// primaryTestIPv4 and secondaryTestIPv4.
	testDomain = "k6.test"

	// primaryTestIPv4 is a default IPv4 address we configure our test DNS server  to resolve the
	// testDomain to.
	//
	// We explicitly use a "martian", non-routable IP address (as per [RFC 1918]) to avoid any potential conflicts with
	// real-world IP addresses.
	//
	// [RFC 1918]: https://datatracker.ietf.org/doc/html/rfc1918
	primaryTestIPv4 = "203.0.113.1"

	// primaryTestIPv6 is a default IPv6 address we configure our test DNS server  to resolve the
	// testDomain to. This points to the same IP as primaryTestIPv4, and is subject to the same routing
	// constraints.
	primaryTestIPv6 = "fd60:76ff:fe12:3456:789a:bcde:f012:3456"

	// secondaryTestIPv4 is a default IP address we configure our test DNS server  to resolve the
	// testDomain to.
	//
	// We explicitly use a "martian", non-routable IP address (as per [RFC 1918]) to avoid any potential conflicts with
	// real-world IP addresses.
	//
	// [RFC 1918]: https://datatracker.ietf.org/doc/html/rfc1918
	secondaryTestIPv4 = "203.0.113.11"

	// secondaryTestIPv6 is a default IPv6 address we configure our test DNS server  to resolve the
	// testDomain to. This points to the same IP as secondaryTestIPv4, and is subject to the same routing
	// constraints.
	secondaryTestIPv6 = "fd61:76ff:fe12:3456:789a:bcde:f012:6789"

	// primaryTestTXT is a TXT record value we configure our test DNS server to resolve
	// the testDomain to.
	primaryTestTXT = "v=spf1 include:example.com ~all"
)

func TestClient_Resolve(t *testing.T) {
	t.Parallel()

	t.Run("Resolving in the init context should fail", func(t *testing.T) {
		t.Parallel()

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(`
			await dns.resolve("k6.io", "A", "1.1.1.1:53");
		`))

		assert.Error(t, err)
	})

	t.Run("Resolving existing A records against cloudflare nameserver should succeed", func(t *testing.T) {
		t.Parallel()

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Setting up the runtime with the necessary state
		runtime.MoveToVUContext(newTestVUState())

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(`
			const resolveResults = await dns.resolve("k6.io", "A", "1.1.1.1:53");
		
			if (resolveResults.length === 0) {
				throw "Resolving k6.io against cloudflare CDN returned no results, expected at least one IP"
			}
		`))

		assert.NoError(t, err)
	})

	t.Run("Resolving existing A records against test nameserver should succeed", func(t *testing.T) {
		t.Parallel()

		port, _ := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Setting up the runtime with the necessary state to execute in the VU context
		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			const resolveResults = await dns.resolve(
				"` + testDomain + `",
				"` + RecordTypeA.String() + `",
				"127.0.0.1:` + port + `"
			);
		
			if (resolveResults.length === 0) {
				throw "Resolving k6.local against unbound server test container returned no results, expected ['` + primaryTestIPv4 + `']"
			}
			
			if (resolveResults.length !== 2) {
				throw "Resolving k6.local against unbound server test container returned an unexpected number of results, expected 2 ips, got:" + resolveResults.length
			}
		
			// We sort the results to ensure that the order is consistent
			// and we can compare the results with the expected values
			resolveResults.sort();

		
			if (resolveResults[0] !== "` + primaryTestIPv4 + `") {
				throw "Resolving k6.local against unbound server test container returned unexpected result, expected '` + primaryTestIPv4 + `', got " + resolveResults[0]
			}
		
			if (resolveResults[1] !== "` + secondaryTestIPv4 + `") {
				throw "Resolving k6.local against unbound server test container returned unexpected result, expected '` + secondaryTestIPv4 + `', got " + resolveResults[1]
			}
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving non-existing A records against test nameserver should succeed", func(t *testing.T) {
		t.Parallel()

		port, _ := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Setting up the runtime with the necessary state to execute in the VU context
		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			try {
				const resolvedResults = await dns.resolve(
					"missing.domain",
					"` + RecordTypeA.String() + `",
					"127.0.0.1:` + port + `"
				);
			} catch (err) {
				if (err.name !== "NonExistingDomain") {
					throw "Resolving missing.domain against unbound server test container returned unexpected error, expected NonExistingDomain, got: " + err.Name
				}
		
				// We expected this error, so we can return
				return
			}
		
			throw "Resolving missing.domain against unbound server test container should have thrown an error, but it didn't"
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving existing AAAA records against test nameserver should succeed", func(t *testing.T) {
		t.Parallel()

		port, _ := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Setting up the runtime with the necessary state to execute in the VU context
		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			const resolveResults = await dns.resolve(
				"` + testDomain + `",
				"` + RecordTypeAAAA.String() + `",
				"127.0.0.1:` + port + `"
			);
		
			// We sort the results to ensure that the order is consistent
			// and we can compare the results with the expected values
			resolveResults.sort();
		
			if (resolveResults.length === 0) {
				throw "Resolving k6.local against unbound server test container returned no results, expected ['` + primaryTestIPv6 + `']"
			}
			
			if (resolveResults.length !== 2) {
				throw "Resolving k6.local against unbound server test container returned an unexpected number of results, expected 2 ips, got:" + resolveResults.length
			}
		
			if (resolveResults[0] !== "` + primaryTestIPv6 + `") {
				throw "Resolving k6.local against unbound server test container returned unexpected result, expected '` + primaryTestIPv6 + `', got " + resolveResults[0]
			}
		
			if (resolveResults[1] !== "` + secondaryTestIPv6 + `") {
				throw "Resolving k6.local against unbound server test container returned unexpected result, expected '` + secondaryTestIPv6 + `', got " + resolveResults[1]
			}
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving non-existing AAAA records against test nameserver should succeed", func(t *testing.T) {
		t.Parallel()

		port, _ := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Setting up the runtime with the necessary state to execute in the VU context
		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			try {
				const resolvedResults = await dns.resolve(
					"missing.domain",
					"` + RecordTypeAAAA.String() + `",
					"127.0.0.1:` + port + `"
				);
			} catch (err) {
				if (err.name !== "NonExistingDomain") {
					throw "Resolving missing.domain against unbound server test container returned unexpected error, expected NonExistingDomain, got: " + err.name
				}
				
				// We expected this error, so we can return
				return
			}
		
			throw "Resolving missing.domain against unbound server test container should have thrown an error, but it didn't"
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving existing TXT records against test nameserver should succeed", func(t *testing.T) {
		t.Parallel()

		port, _ := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			const resolveResults = await dns.resolve(
				"` + testDomain + `",
				"` + RecordTypeTXT.String() + `",
				"127.0.0.1:` + port + `"
			);
		
			if (resolveResults.length === 0) {
				throw "Resolving ` + testDomain + ` TXT against test nameserver returned no results, expected at least one TXT record"
			}
		
			if (resolveResults[0] !== "` + primaryTestTXT + `") {
				throw "Resolving ` + testDomain + ` TXT against test nameserver returned unexpected result, expected '` + primaryTestTXT + `', got " + resolveResults[0]
			}
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving non-existing TXT records against test nameserver should succeed", func(t *testing.T) {
		t.Parallel()

		port, _ := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			try {
				const resolvedResults = await dns.resolve(
					"missing.domain",
					"` + RecordTypeTXT.String() + `",
					"127.0.0.1:` + port + `"
				);
			} catch (err) {
				if (err.name !== "NonExistingDomain") {
					throw "Resolving missing.domain TXT against test nameserver returned unexpected error, expected NonExistingDomain, got: " + err.name
				}
		
				return
			}
		
			throw "Resolving missing.domain TXT against test nameserver should have thrown an error, but it didn't"
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving against a blacklisted IP should fail", func(t *testing.T) {
		t.Parallel()

		// No need to start an Unbound container; we target localhost:53 and rely on the
		// k6 dialer blacklist to block the connection.

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Configure a dialer blacklisting 127.0.0.1
		state := newTestVUState()
		state.Dialer = newTestBlacklistIPsDialer("127.0.0.1", net.CIDRMask(32, 32))
		runtime.MoveToVUContext(state)

		testScript := `
			await dns.resolve(
				"google.com",
				"` + RecordTypeA.String() + `",
				"127.0.0.1:53"
			);
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.Error(t, err)
	})

	t.Run("Resolving against a blocked hostname should fail", func(t *testing.T) {
		t.Parallel()

		// No need to start an Unbound container; we target localhost:53 and rely on the
		// k6 dialer blacklist to block the connection.

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		state := newTestVUState()
		state.Dialer = newTestBlockedHostnameDialer("blocked.com")
		runtime.MoveToVUContext(state)

		testScript := `
			try {
				await dns.resolve(
					"blocked.com",
					"` + RecordTypeA.String() + `",
					"127.0.0.1:53"
				);
			} catch (err) {
				if (err.name !== "BlockedHostname") {
					throw "Resolving blocked.com against unbound server test container returned unexpected error, expected BlockedHostname, got: " + err.name
				}
				
				// We expected this error, so we can return
				return
			}

			throw "Resolving blocked.com against unbound server test container should have thrown an error, but it didn't"
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})
}

func TestClient_ResolveIPv6Nameservers(t *testing.T) {
	t.Parallel()

	t.Run("Resolving using bare IPv6 loopback nameserver address should succeed", func(t *testing.T) {
		t.Parallel()

		// No server needed — the test only verifies the nameserver string
		// parses; the dial against ::1:53 fails locally, which is fine.

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		// Test using bare IPv6 address without port (should default to port 53)
		// Note: We can't use ::1 directly because the container is on IPv4 localhost
		// This test validates the parsing logic handles IPv6 format correctly
		testScript := `
			try {
				// This tests that we can parse IPv6 addresses
				// Using a real IPv6 address would require IPv6 network setup
				await dns.resolve(
					"` + testDomain + `",
					"` + RecordTypeAAAA.String() + `",
					"::1"
				);
			} catch (err) {
				// We expect a connection error since ::1 won't have our test server
				// but the parse should succeed. If parsing fails, we get a different error.
				if (err.message && err.message.includes("invalid nameserver")) {
					throw "IPv6 parsing failed: " + err.message;
				}
				// Connection failure is expected - parsing succeeded
				return;
			}
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving using IPv6 nameserver with bracket notation and port should succeed", func(t *testing.T) {
		t.Parallel()

		_, ipv6Port := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			const resolveResults = await dns.resolve(
				"` + testDomain + `",
				"` + RecordTypeAAAA.String() + `",
				"[::1]:` + ipv6Port + `"
			);

			if (resolveResults.length !== 2) {
				throw "expected 2 AAAA records, got " + resolveResults.length;
			}
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving using bracketed IPv6 nameserver without port should succeed", func(t *testing.T) {
		t.Parallel()

		// No server needed — the test only verifies the nameserver string
		// parses; the dial against ::1:53 fails locally, which is fine.

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			try {
				await dns.resolve(
					"` + testDomain + `",
					"` + RecordTypeAAAA.String() + `",
					"[::1]"
				);
			} catch (err) {
				if (err.message && err.message.includes("invalid nameserver")) {
					throw "IPv6 bracketed without port parsing failed: " + err.message;
				}
				// Connection failure is expected - parsing succeeded
				return;
			}
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving using compressed IPv6 notation should succeed", func(t *testing.T) {
		t.Parallel()

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			try {
				// Test compressed IPv6 notation parsing
				await dns.resolve(
					"k6.io",
					"` + RecordTypeAAAA.String() + `",
					"fe80::1"
				);
			} catch (err) {
				if (err.message && err.message.includes("invalid nameserver")) {
					throw "Compressed IPv6 notation parsing failed: " + err.message;
				}
				// Connection failure is expected - parsing succeeded
				return;
			}
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving AAAA records using IPv6 nameserver should succeed", func(t *testing.T) {
		t.Parallel()

		_, ipv6Port := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			const resolveResults = await dns.resolve(
				"` + testDomain + `",
				"` + RecordTypeAAAA.String() + `",
				"[::1]:` + ipv6Port + `"
			);

			if (resolveResults.length !== 2) {
				throw "expected 2 AAAA records, got " + resolveResults.length;
			}
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving AAAA records using IPv6 nameserver with brackets and port should succeed", func(t *testing.T) {
		t.Parallel()

		_, ipv6Port := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			const resolveResults = await dns.resolve(
				"` + testDomain + `",
				"` + RecordTypeAAAA.String() + `",
				"[::1]:` + ipv6Port + `"
			);

			if (resolveResults.length !== 2) {
				throw "expected 2 AAAA records, got " + resolveResults.length;
			}
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving A records using IPv6 nameserver should succeed", func(t *testing.T) {
		t.Parallel()

		_, ipv6Port := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		// Asking for A records via an IPv6 nameserver — checks both code paths
		// (socket family v6, response type v4).
		testScript := `
			const resolveResults = await dns.resolve(
				"` + testDomain + `",
				"` + RecordTypeA.String() + `",
				"[::1]:` + ipv6Port + `"
			);

			if (resolveResults.length !== 2) {
				throw "expected 2 A records, got " + resolveResults.length;
			}
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving non-existing domain against IPv6 nameserver should return NonExistingDomain error", func(t *testing.T) {
		t.Parallel()

		_, ipv6Port := startTestDNSServer(t)

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		testScript := `
			try {
				await dns.resolve(
					"missing.domain",
					"` + RecordTypeAAAA.String() + `",
					"[::1]:` + ipv6Port + `"
				);
			} catch (err) {
				if (err.name !== "NonExistingDomain") {
					throw "Expected NonExistingDomain error, got: " + err.name;
				}
				return;
			}

			throw "Expected NonExistingDomain error but query succeeded";
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving against blacklisted IPv6 nameserver should fail", func(t *testing.T) {
		t.Parallel()

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Configure a dialer blacklisting the IPv6 loopback address
		state := newTestVUState()
		state.Dialer = newTestBlacklistIPsDialer("::1", net.CIDRMask(128, 128))
		runtime.MoveToVUContext(state)

		testScript := `
			await dns.resolve(
				"k6.io",
				"` + RecordTypeAAAA.String() + `",
				"[::1]:53"
			);
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "blacklisted")
	})

	t.Run("Resolving blocked hostname against IPv6 nameserver should fail", func(t *testing.T) {
		t.Parallel()

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		state := newTestVUState()
		state.Dialer = newTestBlockedHostnameDialer("blocked.com")
		runtime.MoveToVUContext(state)

		testScript := `
			try {
				await dns.resolve(
					"blocked.com",
					"` + RecordTypeAAAA.String() + `",
					"[::1]:53"
				);
			} catch (err) {
				if (err.name !== "BlockedHostname") {
					throw "Expected BlockedHostname error, got: " + err.name;
				}

				// Expected error received
				return;
			}

			throw "Expected BlockedHostname error but query succeeded";
		`

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
		assert.NoError(t, err)
	})

	t.Run("Resolving using malformed IPv6 nameserver should fail with clear error", func(t *testing.T) {
		t.Parallel()

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		runtime.MoveToVUContext(newTestVUState())

		testCases := []struct {
			name       string
			nameserver string
		}{
			{"invalid IPv6", "gggg::1"},
			{"missing bracket close", "[::1:53"},
			{"incomplete IPv6", "2606:4700:"},
			{"port without brackets", "::1:99999"},
		}

		for _, tc := range testCases {
			testScript := `
				try {
					await dns.resolve(
						"k6.io",
						"` + RecordTypeAAAA.String() + `",
						"` + tc.nameserver + `"
					);
				} catch (err) {
					const errMsg = (err.message || err.toString());

					// We expect a parsing error
					if (
						errMsg.includes("invalid nameserver") ||
						errMsg.includes("parsing nameserver")
					) {
						// Expected error
						return;
					}

					throw "Expected parsing error for malformed IPv6, got: " + errMsg;
				}

				throw "Expected parsing error but query succeeded";
			`

			_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(testScript))
			assert.NoError(t, err, "Test case: %s", tc.name)
		}
	})
}

func TestClient_Lookup(t *testing.T) {
	t.Parallel()

	t.Run("Lookup fails in the init context", func(t *testing.T) {
		t.Parallel()

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		_, err = runtime.RunOnEventLoop(wrapInAsyncLambda(`
			await dns.lookup("k6.io");
		`))

		// network operations are forbidden in the init context, thus
		// we explicitly expect an error here
		assert.Error(t, err)
	})

	t.Run("Lookup with a nil dialer should fail", func(t *testing.T) {
		t.Parallel()

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Setting up the runtime with the necessary state
		state := newTestVUState()
		state.Dialer = nil
		runtime.MoveToVUContext(state)

		_, gotErr := runtime.RunOnEventLoop(wrapInAsyncLambda(`
			await dns.lookup("k6.io");
		`))

		assert.Error(t, gotErr)
	})

	t.Run("Lookup against a blocked hostname should fail", func(t *testing.T) {
		t.Parallel()

		runtime, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		state := newTestVUState()
		state.Dialer = newTestBlockedHostnameDialer("blocked.com")
		runtime.MoveToVUContext(state)

		_, gotErr := runtime.RunOnEventLoop(wrapInAsyncLambda(`
			try {
				await dns.lookup("blocked.com");
			} catch (err) {
				if (err.name !== "BlockedHostname") {
					throw "Looking up blocked.com against unbound server test container returned unexpected error, expected BlockedHostname, got: " + err.name
				}
				
				// We expected this error, so we can return
				return
			}

			throw "Looking up blocked.com against unbound server test container should have thrown an error, but it didn't"
		`))

		assert.NoError(t, gotErr)
	})
}

const initGlobals = `
	globalThis.dns = require("k6/x/dns");
`

type testRuntime struct{ *extensionapitest.Runtime }

func newConfiguredRuntime(t testing.TB) (*testRuntime, error) {
	t.Helper()
	runtime := &testRuntime{Runtime: extensionapitest.NewRuntime()}
	dns := New().NewModuleInstance(runtime.VU).Exports().Named
	if err := runtime.VU.Runtime().Set("require", func(path string) any {
		if path != ImportPath {
			panic("unexpected module: " + path)
		}
		return dns
	}); err != nil {
		return nil, err
	}
	if _, err := runtime.VU.Runtime().RunString(initGlobals); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (r *testRuntime) MoveToVUContext(state *testVUState) {
	if state == nil || state.Dialer == nil {
		r.VU.DialContextFunc = nil
		r.VU.LookupHostFunc = nil
		r.VU.CheckHostFunc = nil
		return
	}
	r.VU.DialContextFunc = state.Dialer.DialContext
	r.VU.LookupHostFunc = state.Dialer.LookupHost
	r.VU.CheckHostFunc = state.Dialer.CheckHost
}

func (r *testRuntime) RunOnEventLoop(source string) (sobek.Value, error) {
	var result sobek.Value
	err := r.EventLoop.Start(func() error {
		var err error
		result, err = r.VU.Runtime().RunString(source)
		return err
	})
	return result, err
}

// wrapInAsyncLambda is a helper function that wraps the provided input in an async lambda. This
// makes the use of `await` statements in the input possible.
func wrapInAsyncLambda(input string) string {
	// This makes it possible to use `await` freely on the "top" level
	return "(async () => {\n " + input + "\n })()"
}

// startTestDNSServer starts small UDP DNS servers on random ports of
// 127.0.0.1 and [::1] sharing the same handler — they answer A/AAAA/TXT queries
// for testDomain with the test IPs and TXT records, and return NXDOMAIN for anything else.
// Both listeners shut down via t.Cleanup.
//
// In-process via miekg/dns; no Docker dependency, works identically on every
// platform that has loopback (which is every platform).
func startTestDNSServer(t *testing.T) (ipv4Port, ipv6Port string) {
	t.Helper()

	records := map[uint16][]string{
		miekgdns.TypeA:    {primaryTestIPv4, secondaryTestIPv4},
		miekgdns.TypeAAAA: {primaryTestIPv6, secondaryTestIPv6},
		miekgdns.TypeTXT:  {primaryTestTXT},
	}

	handler := miekgdns.HandlerFunc(func(w miekgdns.ResponseWriter, r *miekgdns.Msg) {
		m := new(miekgdns.Msg).SetReply(r)
		m.Authoritative = true
		for _, q := range r.Question {
			if !strings.EqualFold(strings.TrimSuffix(q.Name, "."), testDomain) {
				m.Rcode = miekgdns.RcodeNameError
				continue
			}
			hdr := miekgdns.RR_Header{Name: q.Name, Rrtype: q.Qtype, Class: miekgdns.ClassINET, Ttl: 60}
			for _, addr := range records[q.Qtype] {
				switch q.Qtype {
				case miekgdns.TypeA:
					m.Answer = append(m.Answer, &miekgdns.A{Hdr: hdr, A: net.ParseIP(addr).To4()})
				case miekgdns.TypeAAAA:
					m.Answer = append(m.Answer, &miekgdns.AAAA{Hdr: hdr, AAAA: net.ParseIP(addr).To16()})
				case miekgdns.TypeTXT:
					m.Answer = append(m.Answer, &miekgdns.TXT{Hdr: hdr, Txt: []string{addr}})
				}
			}
		}
		_ = w.WriteMsg(m)
	})

	listen := func(network, addr string) string {
		pc, err := (&net.ListenConfig{}).ListenPacket(context.Background(), network, addr)
		require.NoError(t, err)
		srv := &miekgdns.Server{PacketConn: pc, Handler: handler}
		started := make(chan struct{})
		srv.NotifyStartedFunc = func() { close(started) }
		go func() { _ = srv.ActivateAndServe() }()
		<-started
		t.Cleanup(func() { _ = srv.Shutdown() })
		_, port, _ := net.SplitHostPort(pc.LocalAddr().String())
		return port
	}

	return listen("udp4", "127.0.0.1:0"), listen("udp6", "[::1]:0")
}

type testVUState struct{ Dialer *testDialer }

type testDialer struct {
	DialContext func(context.Context, string, string) (net.Conn, error)
	LookupHost  func(context.Context, string) ([]string, error)
	CheckHost   func(context.Context, string) error
}

func newTestVUState() *testVUState {
	return &testVUState{Dialer: newTestDialer()}
}

func newTestDialer() *testDialer {
	dialer := net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 10 * time.Second,
	}
	return &testDialer{
		DialContext: dialer.DialContext,
		LookupHost:  net.DefaultResolver.LookupHost,
		CheckHost:   func(context.Context, string) error { return nil },
	}
}

func newTestBlacklistIPsDialer(ip string, m net.IPMask) *testDialer {
	dialer := newTestDialer()
	blocked := &net.IPNet{IP: net.ParseIP(ip), Mask: m}
	dial := dialer.DialContext
	dialer.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err == nil && blocked.Contains(net.ParseIP(host)) {
			return nil, &net.OpError{Op: "dial", Net: network, Err: errors.New("IP is blacklisted")}
		}
		return dial(ctx, network, address)
	}
	return dialer
}

func newTestBlockedHostnameDialer(hostname string) *testDialer {
	dialer := newTestDialer()
	dialer.CheckHost = func(_ context.Context, host string) error {
		if strings.EqualFold(strings.TrimSuffix(host, "."), hostname) {
			return extensionapi.ErrNetworkPolicyUnavailable
		}
		return nil
	}
	return dialer
}
