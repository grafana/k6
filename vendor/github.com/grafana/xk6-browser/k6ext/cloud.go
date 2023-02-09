package k6ext

import "os"

// OnCloud returns true if xk6-browser runs in the cloud.
func OnCloud() bool {
	_, ok := os.LookupEnv("K6_CLOUDRUN_INSTANCE_ID")
	return ok
}
