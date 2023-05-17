package browser

import "github.com/grafana/xk6-browser/env"

type remoteRegistry struct {
	isRemoteBrowser bool
	wsURL           string
}

func newRemoteRegistry(envLookup env.LookupFunc) *remoteRegistry {
	r := &remoteRegistry{}
	r.wsURL, r.isRemoteBrowser = env.IsRemoteBrowser(envLookup)
	return r
}

func (r *remoteRegistry) IsRemoteBrowser() (string, bool) {
	return r.wsURL, r.isRemoteBrowser
}
