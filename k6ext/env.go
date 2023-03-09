package k6ext

import (
	"crypto/rand"
	"math/big"
	"strings"
)

type envLookupper func(key string) (string, bool)

// IsRemoteBrowser returns true and the corresponding CDP
// WS URL if this one is set through the K6_BROWSER_WS_URL
// environment variable. Otherwise returns false.
// If K6_BROWSER_WS_URL is set as a comma separated list of
// URLs, this method returns a randomly chosen URL from the list
// so connections are done in a round-robin fashion for all the
// entries in the list.
func IsRemoteBrowser(envLookup envLookupper) (wsURL string, isRemote bool) {
	wsURL, isRemote = envLookup("K6_BROWSER_WS_URL")
	if !isRemote {
		return "", false
	}
	if !strings.ContainsRune(wsURL, ',') {
		return wsURL, isRemote
	}

	// If last parts element is a void string,
	// because WS URL contained an ending comma,
	// remove it
	parts := strings.Split(wsURL, ",")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	// Choose a random WS URL from the provided list
	i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(parts))))
	wsURL = parts[i.Int64()]

	return wsURL, isRemote
}
