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
			// (500 (Kb/s) * 1000 (to bits/s)) / 8 (to bytes/s)) * 0.8 (20% bandwidth loss)
			Download: ((500 * 1000) / 8) * 0.8,
			// (500 (Kb/s) * 1000 (to bits/s)) / 8 (to bytes/s)) * 0.8 (20% bandwidth loss)
			Upload:  ((500 * 1000) / 8) * 0.8,
			Latency: 400 * 5,
		},
		"Fast 3G": {
			// ((1.6 (Mb/s) * 1000 (to Kb/s) * 1000 (to bits/s)) / 8 (to bytes/s)) * 0.9 (10% bandwidth loss)
			Download: ((1.6 * 1000 * 1000) / 8) * 0.9,
			// (750 (Kb/s) * 1000 (to bits/s)) / 8 (to bytes/s)) * 0.9 (10% bandwidth loss)
			Upload:  ((750 * 1000) / 8) * 0.9,
			Latency: 150 * 3.75,
		},
	}
}
