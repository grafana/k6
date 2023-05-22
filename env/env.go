package env

import (
	"strings"
)

// LookupFunc defines a function to look up a key from the environment.
type LookupFunc func(key string) (string, bool)

// IsRemoteBrowser returns true and the corresponding CDP
// WS URLs when set through the K6_BROWSER_WS_URL environment
// variable. Otherwise returns false and nil.
//
// K6_BROWSER_WS_URL can be defined as a single WS URL or a
// comma separated list of URLs.
func IsRemoteBrowser(envLookup LookupFunc) ([]string, bool) {
	wsURL, isRemote := envLookup("K6_BROWSER_WS_URL")
	if !isRemote {
		return nil, false
	}
	if !strings.ContainsRune(wsURL, ',') {
		return []string{wsURL}, isRemote
	}

	// If last parts element is a void string,
	// because WS URL contained an ending comma,
	// remove it
	parts := strings.Split(wsURL, ",")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	return parts, isRemote
}
