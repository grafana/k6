package platform

// init verifies that the current CPU supports the required ARM64 features
func init() {
	// No further checks currently needed.
	archRequirementsVerified = true
}
