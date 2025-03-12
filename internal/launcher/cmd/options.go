package cmd

// Options contains the optional parameters of the Command function.
type Options struct {
	// BuildServiceURL contains the URL of the k6 build service to be used.
	// If the value is not nil, the k6 binary is built using the build service instead of the local build.
	BuildServiceURL string
	// BuildServiceToken contains the token to be used to authenticate with the build service.
	// Defaults to K6_CLOUD_TOKEN environment variable is set, or the value stored in the k6 config file.
	BuildServiceToken string
}
