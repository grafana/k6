package common

// NetworkProfile is used in ThrottleNetwork.
type NetworkProfile struct {
	// Minimum latency from request sent to response headers received (ms).
	Latency float64

	// Maximal aggregated download throughput (bytes/sec). -1 disables download throttling.
	Download float64

	// Maximal aggregated upload throughput (bytes/sec). -1 disables upload throttling.
	Upload float64
}

// NewNetworkProfile creates a non-throttled network profile.
func NewNetworkProfile() NetworkProfile {
	return NetworkProfile{
		Latency:  0,
		Download: -1,
		Upload:   -1,
	}
}

// GetNetworkProfiles returns NetworkProfiles which are ready to be used to
// throttle the network with page.throttleNetwork.
func GetNetworkProfiles() map[string]NetworkProfile {
	return map[string]NetworkProfile{}
}
