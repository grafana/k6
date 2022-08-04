package cloudapi

// URLForResults returns the cloud URL with the test run results.
func URLForResults(refID string, config Config) string {
	return config.WebAppURL.String + "/runs/" + refID
}
