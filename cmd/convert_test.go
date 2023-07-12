package cmd

import (
	"os"
	"regexp"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cmd/tests"
	"go.k6.io/k6/lib/fsext"
)

const testHAR = `
{
	"log": {
		"version": "1.2",
		"creator": {
		"name": "WebInspector",
		"version": "537.36"
		},
		"pages": [
		{
			"startedDateTime": "2018-01-21T19:48:40.432Z",
			"id": "page_2",
			"title": "https://golang.org/",
			"pageTimings": {
			"onContentLoad": 590.3389999875799,
			"onLoad": 1593.1009999476373
			}
		}
		],
		"entries": [
		{
			"startedDateTime": "2018-01-21T19:48:40.587Z",
			"time": 147.5899999756366,
			"request": {
				"method": "GET",
				"url": "https://golang.org/",
				"httpVersion": "http/2.0+quic/39",
				"headers": [
					{
					"name": "pragma",
					"value": "no-cache"
					}
				],
				"queryString": [],
				"cookies": [],
				"headersSize": -1,
				"bodySize": 0
			},
			"cache": {},
			"timings": {
				"blocked": 0.43399997614324004,
				"dns": -1,
				"ssl": -1,
				"connect": -1,
				"send": 0.12700003571808005,
				"wait": 149.02899996377528,
				"receive": 0,
				"_blocked_queueing": -1
			},
			"serverIPAddress": "172.217.22.177",
			"pageref": "page_2"
		}
		]
	}
}
`

const testHARConvertResult = `import { group, sleep } from 'k6';
import http from 'k6/http';

// Version: 1.2
// Creator: WebInspector

export let options = {
    maxRedirects: 0,
};

export default function() {

	group("page_2 - https://golang.org/", function() {
		let req, res;
		req = [{
			"method": "get",
			"url": "https://golang.org/",
			"params": {
				"headers": {
					"pragma": "no-cache"
				}
			}
		}];
		res = http.batch(req);
		// Random sleep between 20s and 40s
		sleep(Math.floor(Math.random()*20+20));
	});

}
`

func TestConvertCmdCorrelate(t *testing.T) {
	t.Parallel()
	har, err := os.ReadFile("testdata/example.har") //nolint:forbidigo
	require.NoError(t, err)

	expectedTestPlan, err := os.ReadFile("testdata/example.js") //nolint:forbidigo
	require.NoError(t, err)

	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, "correlate.har", har, 0o644))
	ts.CmdArgs = []string{
		"k6", "convert", "--output=result.js", "--correlate=true", "--no-batch=true",
		"--enable-status-code-checks=true", "--return-on-failed-check=true", "correlate.har",
	}

	newRootCommand(ts.GlobalState).execute()

	result, err := fsext.ReadFile(ts.FS, "result.js")
	require.NoError(t, err)

	// Sanitizing to avoid windows problems with carriage returns
	re := regexp.MustCompile(`\r`)
	expected := re.ReplaceAllString(string(expectedTestPlan), ``)
	resultStr := re.ReplaceAllString(string(result), ``)

	if assert.NoError(t, err) {
		// assert.Equal suppresses the diff it is too big, so we add it as the test error message manually as well.
		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(expected),
			B:        difflib.SplitLines(resultStr),
			FromFile: "Expected",
			FromDate: "",
			ToFile:   "Actual",
			ToDate:   "",
			Context:  1,
		})

		assert.Equal(t, expected, resultStr, diff)
	}
}

func TestConvertCmdStdout(t *testing.T) {
	t.Parallel()
	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, "stdout.har", []byte(testHAR), 0o644))
	ts.CmdArgs = []string{"k6", "convert", "stdout.har"}

	newRootCommand(ts.GlobalState).execute()
	assert.Equal(t, "Command \"convert\" is deprecated, please use har-to-k6 (https://github.com/grafana/har-to-k6) instead.\n"+testHARConvertResult, ts.Stdout.String())
}

func TestConvertCmdOutputFile(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, "output.har", []byte(testHAR), 0o644))
	ts.CmdArgs = []string{"k6", "convert", "--output", "result.js", "output.har"}

	newRootCommand(ts.GlobalState).execute()

	output, err := fsext.ReadFile(ts.FS, "result.js")
	assert.NoError(t, err)
	assert.Equal(t, testHARConvertResult, string(output))
}

// TODO: test options injection; right now that's difficult because when there are multiple
// options, they can be emitted in different order in the JSON
