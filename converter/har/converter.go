package har

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/tidwall/pretty"

	"go.k6.io/k6/lib"
)

// fprint panics when where's an error writing to the supplied io.Writer
// since this will be used on in-memory expandable buffers, that should
// happen only when we run out of memory...
func fprint(w io.Writer, a ...interface{}) int {
	n, err := fmt.Fprint(w, a...)
	if err != nil {
		panic(err.Error())
	}
	return n
}

// fprintf panics when where's an error writing to the supplied io.Writer
// since this will be used on in-memory expandable buffers, that should
// happen only when we run out of memory...
func fprintf(w io.Writer, format string, a ...interface{}) int {
	n, err := fmt.Fprintf(w, format, a...)
	if err != nil {
		panic(err.Error())
	}
	return n
}

// TODO: refactor this to have fewer parameters... or just refactor in general...
func Convert(h HAR, options lib.Options, minSleep, maxSleep uint, enableChecks bool, returnOnFailedCheck bool, batchTime uint, nobatch bool, correlate bool, only, skip []string) (result string, convertErr error) {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)

	if returnOnFailedCheck && !enableChecks {
		return "", fmt.Errorf("return on failed check requires --enable-status-code-checks")
	}

	if correlate && !nobatch {
		return "", fmt.Errorf("correlation requires --no-batch")
	}

	if h.Log == nil {
		return "", fmt.Errorf("invalid HAR file supplied, the 'log' property is missing")
	}

	if enableChecks {
		fprint(w, "import { group, check, sleep } from 'k6';\n")
	} else {
		fprint(w, "import { group, sleep } from 'k6';\n")
	}
	fprint(w, "import http from 'k6/http';\n\n")

	fprintf(w, "// Version: %v\n", h.Log.Version)
	fprintf(w, "// Creator: %v\n", h.Log.Creator.Name)
	if h.Log.Browser != nil {
		fprintf(w, "// Browser: %v\n", h.Log.Browser.Name)
	}
	if h.Log.Comment != "" {
		fprintf(w, "// %v\n", h.Log.Comment)
	}

	fprint(w, "\nexport let options = {\n")
	options.ForEachSpecified("json", func(key string, val interface{}) {
		if valJSON, err := json.MarshalIndent(val, "    ", "    "); err != nil {
			convertErr = err
		} else {
			fprintf(w, "    %s: %s,\n", key, valJSON)
		}
	})
	if convertErr != nil {
		return "", convertErr
	}
	fprint(w, "};\n\n")

	fprint(w, "export default function() {\n\n")

	pages := h.Log.Pages
	sort.Sort(PageByStarted(pages))

	// Hack to handle HAR files without a pages array
	// Temporary fix for https://github.com/k6io/k6/issues/793
	if len(pages) == 0 {
		pages = []Page{{
			ID:      "", // The Pageref property of all Entries will be an empty string
			Title:   "Global",
			Comment: "Placeholder page since there were no pages specified in the HAR file",
		}}
	}

	// Grouping by page and URL filtering
	pageEntries := make(map[string][]*Entry)
	for _, e := range h.Log.Entries {

		// URL filtering
		u, err := url.Parse(e.Request.URL)
		if err != nil {
			return "", err
		}
		if !IsAllowedURL(u.Host, only, skip) {
			continue
		}

		// Avoid multipart/form-data requests until k6 scripts can support binary data
		if e.Request.PostData != nil && strings.HasPrefix(e.Request.PostData.MimeType, "multipart/form-data") {
			continue
		}

		// Create new group o adding page to a existing one
		if _, ok := pageEntries[e.Pageref]; !ok {
			pageEntries[e.Pageref] = append([]*Entry{}, e)
		} else {
			pageEntries[e.Pageref] = append(pageEntries[e.Pageref], e)
		}
	}

	for i, page := range pages {

		entries := pageEntries[page.ID]

		scriptGroupName := page.ID + " - " + page.Title
		if page.ID == "" {
			// Temporary fix for https://github.com/k6io/k6/issues/793
			// I can't just remove the group() call since all of the subsequent code indentation is hardcoded...
			scriptGroupName = page.Title
		}
		fprintf(w, "\tgroup(%q, function() {\n", scriptGroupName)

		sort.Sort(EntryByStarted(entries))

		if nobatch {
			var recordedRedirectURL string
			previousResponse := map[string]interface{}{}

			fprint(w, "\t\tlet res, redirectUrl, json;\n")

			for entryIndex, e := range entries {

				var params []string
				var cookies []string
				var body string

				fprintf(w, "\t\t// Request #%d\n", entryIndex)

				if e.Request.PostData != nil {
					body = e.Request.PostData.Text
				}

				for _, c := range e.Request.Cookies {
					cookies = append(cookies, fmt.Sprintf(`%q: %q`, c.Name, c.Value))
				}
				if len(cookies) > 0 {
					params = append(params, fmt.Sprintf("\"cookies\": {\n\t\t\t\t%s\n\t\t\t}", strings.Join(cookies, ",\n\t\t\t\t\t")))
				}

				if headers := buildK6Headers(e.Request.Headers); len(headers) > 0 {
					params = append(params, fmt.Sprintf("\"headers\": {\n\t\t\t\t\t%s\n\t\t\t\t}", strings.Join(headers, ",\n\t\t\t\t\t")))
				}

				fprintf(w, "\t\tres = http.%s(", strings.ToLower(e.Request.Method))

				if correlate && recordedRedirectURL != "" {
					if recordedRedirectURL != e.Request.URL {
						return "", errors.New( //nolint:stylecheck
							"The har file contained a redirect but the next request did not match that redirect. " +
								"Possibly a misbehaving client or concurrent requests?",
						)
					}
					fprintf(w, "redirectUrl")
					recordedRedirectURL = ""
				} else {
					fprintf(w, "%q", e.Request.URL)
				}

				if e.Request.Method != "GET" {
					if correlate && e.Request.PostData != nil && strings.Contains(e.Request.PostData.MimeType, "json") {
						requestMap := map[string]interface{}{}

						escapedPostdata := strings.Replace(e.Request.PostData.Text, "$", "\\$", -1)

						if err := json.Unmarshal([]byte(escapedPostdata), &requestMap); err != nil {
							return "", err
						}

						if len(previousResponse) != 0 {
							traverseMaps(requestMap, previousResponse, nil)
						}
						requestText, err := json.Marshal(requestMap)
						if err == nil {
							prettyJSONString := string(pretty.PrettyOptions(requestText, &pretty.Options{Width: 999999, Prefix: "\t\t\t", Indent: "\t", SortKeys: true})[:])
							fprintf(w, ",\n\t\t\t`%s`", strings.TrimSpace(prettyJSONString))
						} else {
							return "", err
						}

					} else {
						fprintf(w, ",\n\t\t%q", body)
					}
				}

				if len(params) > 0 {
					fprintf(w, ",\n\t\t\t{\n\t\t\t\t%s\n\t\t\t}", strings.Join(params, ",\n\t\t\t"))
				}

				fprintf(w, "\n\t\t)\n")

				if e.Response != nil {
					// the response is nil if there is a failed request in the recording, or if responses were not recorded
					if enableChecks {
						if e.Response.Status > 0 {
							if returnOnFailedCheck {
								fprintf(w, "\t\tif (!check(res, {\"status is %v\": (r) => r.status === %v })) { return };\n", e.Response.Status, e.Response.Status)
							} else {
								fprintf(w, "\t\tcheck(res, {\"status is %v\": (r) => r.status === %v });\n", e.Response.Status, e.Response.Status)
							}
						}
					}

					if e.Response.Headers != nil {
						for _, header := range e.Response.Headers {
							if header.Name == "Location" {
								fprintf(w, "\t\tredirectUrl = res.headers.Location;\n")
								recordedRedirectURL = header.Value
								break
							}
						}
					}

					responseMimeType := e.Response.Content.MimeType
					if correlate &&
						strings.Index(responseMimeType, "application/") == 0 &&
						strings.Index(responseMimeType, "json") == len(responseMimeType)-4 {
						if err := json.Unmarshal([]byte(e.Response.Content.Text), &previousResponse); err != nil {
							return "", err
						}
						fprint(w, "\t\tjson = JSON.parse(res.body);\n")
					}
				}
			}
		} else {
			batches := SplitEntriesInBatches(entries, batchTime)

			fprint(w, "\t\tlet req, res;\n")

			for j, batchEntries := range batches {

				fprint(w, "\t\treq = [")
				for k, e := range batchEntries {
					r, err := buildK6RequestObject(e.Request)
					if err != nil {
						return "", err
					}
					fprintf(w, "%v", r)
					if k != len(batchEntries)-1 {
						fprint(w, ",")
					}
				}
				fprint(w, "];\n")
				fprint(w, "\t\tres = http.batch(req);\n")

				if enableChecks {
					for k, e := range batchEntries {
						if e.Response.Status > 0 {
							if returnOnFailedCheck {
								fprintf(w, "\t\tif (!check(res, {\"status is %v\": (r) => r.status === %v })) { return };\n", e.Response.Status, e.Response.Status)
							} else {
								fprintf(w, "\t\tcheck(res[%v], {\"status is %v\": (r) => r.status === %v });\n", k, e.Response.Status, e.Response.Status)
							}
						}
					}
				}

				if j != len(batches)-1 {
					lastBatchEntry := batchEntries[len(batchEntries)-1]
					firstBatchEntry := batches[j+1][0]
					t := firstBatchEntry.StartedDateTime.Sub(lastBatchEntry.StartedDateTime).Seconds()
					fprintf(w, "\t\tsleep(%.2f);\n", t)
				}
			}

			if i == len(pages)-1 {
				// Last page; add random sleep time at the group completion
				fprintf(w, "\t\t// Random sleep between %ds and %ds\n", minSleep, maxSleep)
				fprintf(w, "\t\tsleep(Math.floor(Math.random()*%d+%d));\n", maxSleep-minSleep, minSleep)
			} else {
				// Add sleep time at the end of the group
				nextPage := pages[i+1]
				sleepTime := 0.5
				if len(entries) > 0 {
					lastEntry := entries[len(entries)-1]
					t := nextPage.StartedDateTime.Sub(lastEntry.StartedDateTime).Seconds()
					if t >= 0.01 {
						sleepTime = t
					}
				}
				fprintf(w, "\t\tsleep(%.2f);\n", sleepTime)
			}
		}

		fprint(w, "\t});\n")
	}

	fprint(w, "\n}\n")
	if err := w.Flush(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func buildK6RequestObject(req *Request) (string, error) {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)

	fprint(w, "{\n")

	method := strings.ToLower(req.Method)
	if method == "delete" {
		method = "del"
	}
	fprintf(w, `"method": %q, "url": %q`, method, req.URL)

	if req.PostData != nil && method != "get" {
		postParams, plainText, err := buildK6Body(req)
		if err != nil {
			return "", err
		} else if len(postParams) > 0 {
			fprintf(w, `, "body": { %s }`, strings.Join(postParams, ", "))
		} else if plainText != "" {
			fprintf(w, `, "body": %q`, plainText)
		}
	}

	var params []string
	var cookies []string
	for _, c := range req.Cookies {
		cookies = append(cookies, fmt.Sprintf(`%q: %q`, c.Name, c.Value))
	}
	if len(cookies) > 0 {
		params = append(params, fmt.Sprintf(`"cookies": { %s }`, strings.Join(cookies, ", ")))
	}

	if headers := buildK6Headers(req.Headers); len(headers) > 0 {
		params = append(params, fmt.Sprintf(`"headers": { %s }`, strings.Join(headers, ", ")))
	}

	if len(params) > 0 {
		fprintf(w, `, "params": { %s }`, strings.Join(params, ", "))
	}

	fprint(w, "}")
	if err := w.Flush(); err != nil {
		return "", err
	}

	var buffer bytes.Buffer
	err := json.Indent(&buffer, b.Bytes(), "\t\t", "\t")
	if err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func buildK6Headers(headers []Header) []string {
	var h []string
	if len(headers) > 0 {
		ignored := map[string]bool{"cookie": true, "content-length": true}
		for _, header := range headers {
			name := strings.ToLower(header.Name)
			_, isIgnored := ignored[name]
			// Avoid SPDY's, duplicated or ignored headers
			if !isIgnored && name[0] != ':' {
				ignored[name] = true
				h = append(h, fmt.Sprintf("%q: %q", header.Name, header.Value))
			}
		}
	}
	return h
}

func buildK6Body(req *Request) ([]string, string, error) {
	var postParams []string
	if req.PostData.MimeType == "application/x-www-form-urlencoded" && len(req.PostData.Params) > 0 {
		for _, p := range req.PostData.Params {
			n, err := url.QueryUnescape(p.Name)
			if err != nil {
				return postParams, "", err
			}
			v, err := url.QueryUnescape(p.Value)
			if err != nil {
				return postParams, "", err
			}
			postParams = append(postParams, fmt.Sprintf(`%q: %q`, n, v))
		}
		return postParams, "", nil
	}
	return postParams, req.PostData.Text, nil
}

func traverseMaps(request map[string]interface{}, response map[string]interface{}, path []interface{}) {
	if response == nil {
		// previous call reached a leaf in the response map so there's no point continuing
		return
	}
	for key, val := range request {
		responseVal := response[key]
		if responseVal == nil {
			// no corresponding value in response map (and the type conversion below would fail so we need an early exit)
			continue
		}
		newPath := append(path, key)
		switch concreteVal := val.(type) {
		case map[string]interface{}:
			traverseMaps(concreteVal, responseVal.(map[string]interface{}), newPath)
		case []interface{}:
			traverseArrays(concreteVal, responseVal.([]interface{}), newPath)
		default:
			if responseVal == val {
				request[key] = jsObjectPath(newPath)
			}
		}
	}
}

func traverseArrays(requestArray []interface{}, responseArray []interface{}, path []interface{}) {
	for i, val := range requestArray {
		newPath := append(path, i)
		if len(responseArray) <= i {
			// requestArray had more entries than responseArray
			break
		}
		responseVal := responseArray[i]
		switch concreteVal := val.(type) {
		case map[string]interface{}:
			traverseMaps(concreteVal, responseVal.(map[string]interface{}), newPath)
		case []interface{}:
			traverseArrays(concreteVal, responseVal.([]interface{}), newPath)
		case string:
			if responseVal == val {
				requestArray[i] = jsObjectPath(newPath)
			}
		default:
			panic(jsObjectPath(newPath))
		}
	}
}

func jsObjectPath(path []interface{}) string {
	s := "${json"
	for _, val := range path {
		// this may cause issues with non-array keys with numeric values. test this later.
		switch concreteVal := val.(type) {
		case int:
			s = s + "[" + fmt.Sprint(concreteVal) + "]"
		case string:
			s = s + "." + concreteVal
		}
	}
	s = s + "}"
	return s
}
