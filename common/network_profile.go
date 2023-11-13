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
	return map[string]NetworkProfile{
		"No Throttling": {
			Download: -1,
			Upload:   -1,
			Latency:  0,
		},
		"Slow 3G": {
			Download: ((500 * 1000) / 8) * 0.8,
			Upload:   ((500 * 1000) / 8) * 0.8,
			Latency:  400 * 5,
		},
		"Fast 3G": {
			Download: ((1.6 * 1000 * 1000) / 8) * 0.9,
			Upload:   ((750 * 1000) / 8) * 0.9,
			Latency:  150 * 3.75,
		},
	}
}
