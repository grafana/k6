package cloudapi

import (
	"fmt"
	"strings"
)

// URLForResults returns the cloud URL with the test run results.
func URLForResults(refID string, config Config) string {
	if config.TestRunDetails.Valid {
		return config.TestRunDetails.String
	}

	return config.WebAppURL.String + "/runs/" + refID
}

// URLForTest returns the cloud URL for a test page.
func URLForTest(testID int64, cfg Config) (string, error) {
	if !cfg.StackURL.Valid || cfg.StackURL.String == "" {
		return "", fmt.Errorf("cannot build test page URL: stack URL is not configured")
	}

	return fmt.Sprintf("%s/a/k6-app/tests/%d", strings.TrimRight(cfg.StackURL.String, "/"), testID), nil
}
