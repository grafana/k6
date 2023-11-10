package common

// GetNetworkProfiles returns NetworkProfiles which are ready to be used to
// throttle the network with page.throttleNetwork.
func GetNetworkProfiles() map[string]NetworkProfile {
	return map[string]NetworkProfile{}
}
