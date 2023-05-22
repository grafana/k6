package browser

import (
	"crypto/rand"
	"math/big"

	"github.com/grafana/xk6-browser/env"
	"github.com/grafana/xk6-browser/registry"
)

type remoteRegistry struct {
	isRemoteBrowser bool
	wsURLs          []string
}

func newRemoteRegistry(envLookup env.LookupFunc) *remoteRegistry {
	r := &remoteRegistry{}
	r.wsURLs, r.isRemoteBrowser = registry.IsRemoteBrowser(envLookup)
	return r
}

// IsRemoteBrowser returns a WS URL and true when a WS URL is defined,
// otherwise it returns an empty string and false. If more than one
// WS URL was registered in newRemoteRegistry, a randomly chosen URL from
// the list in a round-robin fashion is selected and returned.
func (r *remoteRegistry) IsRemoteBrowser() (string, bool) {
	if !r.isRemoteBrowser {
		return "", false
	}

	// Choose a random WS URL from the provided list
	i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(r.wsURLs))))
	wsURL := r.wsURLs[i.Int64()]

	return wsURL, true
}
