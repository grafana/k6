package cloudapi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

func TestURLForResults(t *testing.T) {
	t.Parallel()

	webAppURL := "http://example.com"
	testRunDetails := "http://example-new.com"
	refID := "1234"

	conf := Config{
		WebAppURL: null.NewString(webAppURL, true),
	}

	expected := fmt.Sprintf("%s/runs/%s", webAppURL, refID)
	require.Equal(t, expected, URLForResults(refID, conf))
	conf.TestRunDetails = null.NewString(testRunDetails, true)
	require.Equal(t, testRunDetails, URLForResults(refID, conf))
}

func TestURLForTest(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	_, err := URLForTest(1234, cfg)
	require.EqualError(t, err, "cannot build test page URL: stack URL is not configured")

	cfg.StackURL = null.NewString("https://app.k6.io/", true)
	url, err := URLForTest(1234, cfg)
	require.NoError(t, err)
	require.Equal(t, "https://app.k6.io/a/k6-app/tests/1234", url)
}
