package http

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
)

func TestTLS13Support(t *testing.T) {
	t.Parallel()
	ts := newTestCase(t)
	state := ts.runtime.VU.State()

	ts.tb.Mux.HandleFunc("/tls-version", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		ver := req.TLS.Version
		_, err := fmt.Fprint(resp, lib.SupportedTLSVersionsToString[lib.TLSVersion(ver)])
		require.NoError(t, err)
	}))

	// We don't expect any failed requests
	state.Options.Throw = null.BoolFrom(true)
	state.Options.Apply(lib.Options{TLSVersion: &lib.TLSVersions{Max: tls.VersionTLS13}})

	_, err := ts.runtime.VU.Runtime().RunString(ts.tb.Replacer.Replace(`
		var resp = http.get("HTTPSBIN_URL/tls-version");
		if (resp.body != "tls1.3") {
			throw new Error("unexpected tls version: " + resp.body);
		}
	`))
	require.NoError(t, err)
}
