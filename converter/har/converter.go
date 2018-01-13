/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package har

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

func Convert(h HAR, includeCodeCheck bool, batchTime uint, only, skip []string) (string, error) {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	if includeCodeCheck {
		fmt.Fprint(w, "import { group, check, sleep } from 'k6';\n")
	} else {
		fmt.Fprint(w, "import { group, sleep } from 'k6';\n")
	}
	fmt.Fprint(w, "import http from 'k6/http';\n\n")

	fmt.Fprintf(w, "// Version: %v\n", h.Log.Version)
	fmt.Fprintf(w, "// Creator: %v\n", h.Log.Creator.Name)
	if h.Log.Browser != nil {
		fmt.Fprintf(w, "// Browser: %v\n", h.Log.Browser.Name)
	}
	if h.Log.Comment != "" {
		fmt.Fprintf(w, "// %v\n", h.Log.Comment)
	}

	fmt.Fprint(w, "\n")
	fmt.Fprint(w, "export default function() {\n\n")

	pages := h.Log.Pages
	sort.Sort(PageByStarted(pages))

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
		fmt.Fprintf(w, "\tgroup(\"%s - %s\", function() {\n", page.ID, page.Title)

		sort.Sort(EntryByStarted(entries))
		batches := SplitEntriesInBatches(entries, batchTime)

		fmt.Fprint(w, "\t\tlet req, res;\n")

		for j, batchEntries := range batches {

			fmt.Fprint(w, "\t\treq = [")
			for k, e := range batchEntries {
				r, err := buildK6RequestObject(e.Request)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(w, "%v", r)
				if k != len(batchEntries)-1 {
					fmt.Fprint(w, ",")
				}
			}
			fmt.Fprint(w, "];\n")
			fmt.Fprint(w, "\t\tres = http.batch(req);\n")

			if includeCodeCheck {
				for k, e := range batchEntries {
					if e.Response.Status > 0 {
						fmt.Fprintf(w, "\t\tcheck(res[%v], {\n\t\t\"status is %v\": (r) => r.status === %v,\n\t});\n", k, e.Response.Status, e.Response.Status)
					}
				}
			}

			if j != len(batches)-1 {
				lastBatchEntry := batchEntries[len(batchEntries)-1]
				firstBatchEntry := batches[j+1][0]
				t := firstBatchEntry.StartedDateTime.Sub(lastBatchEntry.StartedDateTime).Seconds()
				fmt.Fprintf(w, "\t\tsleep(%.2f);\n", t)
			}
		}

		if i == len(pages)-1 {
			// Last page; add random sleep time at the group completion
			fmt.Fprint(w, "\t\t// Random sleep between 2s and 4s\n")
			fmt.Fprint(w, "\t\tsleep(Math.floor(Math.random()*3+2));\n")
		} else {
			// Add sleep time at the end of the group
			nextPage := pages[i+1]
			lastEntry := entries[len(entries)-1]
			t := nextPage.StartedDateTime.Sub(lastEntry.StartedDateTime).Seconds()
			if t < 0.01 {
				t = 0.5
			}
			fmt.Fprintf(w, "\t\tsleep(%.2f);\n", t)
		}

		fmt.Fprint(w, "\t});\n")
	}

	fmt.Fprint(w, "\n}\n")
	if err := w.Flush(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func buildK6RequestObject(req *Request) (string, error) {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)

	fmt.Fprint(w, "{\n")

	method := strings.ToLower(req.Method)
	if method == "delete" {
		method = "del"
	}
	fmt.Fprintf(w, `"method": %q, "url": %q`, method, req.URL)

	if req.PostData != nil && req.PostData.Text != "" && method != "get" {
		fmt.Fprintf(w, `, "body": %q`, req.PostData.Text)
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
		fmt.Fprintf(w, `, "params": { %s }`, strings.Join(params, ", "))
	}

	fmt.Fprint(w, "}")
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
		m := make(map[string]Header)
		for _, header := range headers {
			name := strings.ToLower(header.Name)
			_, exists := m[name]
			// Avoid SPDY's, duplicated or cookie headers
			if !exists && name[0] != ':' && name != "cookie" {
				m[strings.ToLower(header.Name)] = header
				h = append(h, fmt.Sprintf("%q: %q", header.Name, header.Value))
			}
		}
	}
	return h
}
