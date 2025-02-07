package sigv4

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildCanonicalHeaders(t *testing.T) {
	t.Parallel()

	serviceName := "mockAPI"
	region := "mock-region"
	endpoint := "https://" + serviceName + "." + region + ".example.com"

	now := time.Now().UTC()
	iSO8601Date := now.Format(timeFormat)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, nil)
	if err != nil {
		t.Fatalf("failed to create request, %v", err)
	}

	req.Header.Set("Host", req.Host)
	req.Header.Set(amzDateKey, iSO8601Date)
	req.Header.Set("InnerSpace", "   inner      space    ")
	req.Header.Set("LeadingSpace", "    leading-space")
	req.Header.Add("MultipleSpace", "no-space")
	req.Header.Add("MultipleSpace", "\ttab-space")
	req.Header.Add("MultipleSpace", "trailing-space    ")
	req.Header.Set("NoSpace", "no-space")
	req.Header.Set("TabSpace", "\ttab-space\t")
	req.Header.Set("TrailingSpace", "trailing-space    ")
	req.Header.Set("WrappedSpace", "   wrapped-space    ")

	wantSignedHeader := "host;innerspace;leadingspace;multiplespace;nospace;tabspace;trailingspace;wrappedspace;x-amz-date"
	wantCanonicalHeader := strings.Join([]string{
		"host:mockAPI.mock-region.example.com",
		"innerspace:inner space",
		"leadingspace:leading-space",
		"multiplespace:no-space,tab-space,trailing-space",
		"nospace:no-space",
		"tabspace:tab-space",
		"trailingspace:trailing-space",
		"wrappedspace:wrapped-space",
		"x-amz-date:" + iSO8601Date,
		"",
	}, "\n")

	gotSignedHeaders, gotCanonicalHeader := buildCanonicalHeaders(req, nil)
	assert.Equal(t, wantSignedHeader, gotSignedHeaders)
	assert.Equal(t, wantCanonicalHeader, gotCanonicalHeader)
}
