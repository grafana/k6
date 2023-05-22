package registry

import (
	"crypto/rand"
	"math/big"
	"strings"

	"github.com/grafana/xk6-browser/env"
)

// RemoteRegistry contains the details of the remote web browsers.
// At the moment it's the WS URLs.
type RemoteRegistry struct {
	isRemoteBrowser bool
	wsURLs          []string
}

// NewRemoteRegistry will create a new RemoteRegistry. This will
// parse the K6_BROWSER_WS_URL env var to retrieve the defined
// list of WS URLs.
//
// K6_BROWSER_WS_URL can be defined as a single WS URL or a
// comma separated list of URLs.
func NewRemoteRegistry(envLookup env.LookupFunc) *RemoteRegistry {
	r := &RemoteRegistry{}

	wsURL, isRemote := envLookup("K6_BROWSER_WS_URL")
	if !isRemote {
		return r
	}

	if !strings.ContainsRune(wsURL, ',') {
		r.isRemoteBrowser = true
		r.wsURLs = []string{wsURL}
		return r
	}

	// If last parts element is a void string,
	// because WS URL contained an ending comma,
	// remove it
	parts := strings.Split(wsURL, ",")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	r.isRemoteBrowser = true
	r.wsURLs = parts

	return r
}

// IsRemoteBrowser returns a WS URL and true when a WS URL is defined,
// otherwise it returns an empty string and false. If more than one
// WS URL was registered in newRemoteRegistry, a randomly chosen URL from
// the list in a round-robin fashion is selected and returned.
func (r *RemoteRegistry) IsRemoteBrowser() (string, bool) {
	if !r.isRemoteBrowser {
		return "", false
	}

	// Choose a random WS URL from the provided list
	i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(r.wsURLs))))
	wsURL := r.wsURLs[i.Int64()]

	return wsURL, true
}
