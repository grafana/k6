// Package browser provides an entry point to the browser extension.
package browser

import (
	"log"
	"net/http"
	_ "net/http/pprof" //nolint:gosec
	"os"

	"github.com/grafana/xk6-browser/browser"

	k6modules "go.k6.io/k6/js/modules"
)

func init() {
	if _, ok := os.LookupEnv("K6_BROWSER_PPROF"); ok {
		go func() {
			address := "localhost:6060"
			log.Println("Starting http debug server", address)
			log.Println(http.ListenAndServe(address, nil)) //nolint:gosec
			// no linted because we don't need to set timeouts for the debug server.
		}()
	}
}

func init() {
	k6modules.Register("k6/x/browser", browser.New())
}
