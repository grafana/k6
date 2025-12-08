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
