package cloudapi

// URLForResults returns the cloud URL with the test run results.
func URLForResults(refID string, config Config) string {
	if config.TestRunDetails.Valid {
		return config.TestRunDetails.String
	}

	return config.WebAppURL.String + "/runs/" + refID
}
