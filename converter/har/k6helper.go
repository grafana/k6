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
        "math/rand"
        "net/url"
        "sort"
        "strings"
        "time"
)

const (
        Get   = "get"
        Post  = "post"
        Del   = "del"
        Put   = "put"
        Patch = "patch"
)

// build a K6 request
func BuildK6Request(method, uri, data string, headers []Header, cookies []Cookie) (string, error) {
        var b bytes.Buffer
        w := bufio.NewWriter(&b)

        // method and url
        method = strings.ToLower(method)
        switch method {
        case Get, Post, Put, Patch:
                fmt.Fprintf(w, "http.%v(\"%v\"", method, uri)
        case "delete":
                fmt.Fprintf(w, "http.del(\"%v\"", uri)
        default:
                fmt.Fprintf(w, "http.request(\"%v\",\"%v\"", method, uri)
        }

        // data
        if data != "" {
                fmt.Fprintf(w, ",\n\t\"%s\"", url.QueryEscape(data))
        } else if method != Get {
                fmt.Fprint(w, ",\n\tnull")
        }

        // Add cookie as header
        c := BuildK6CookiesValues(cookies)
        if c != "" {
                headers = append(headers, Header{Name: "Cookie", Value: c})
        }

        if header := BuildK6Headers(headers); len(header) > 0 {
                fmt.Fprintf(w, ",\n\t{ %v }", header)
        }

        fmt.Fprint(w, "\n);\n")

        if err := w.Flush(); err != nil {
                return "", err
        }
        return b.String(), nil
}

// Build a K6 request object for batch requests
func BuildK6RequestObject(method, uri, data string, headers []Header, cookies []Cookie) (string, error) {
        var b bytes.Buffer
        w := bufio.NewWriter(&b)

        fmt.Fprint(w, "{\n")

        method = strings.ToLower(method)
        if method == "delete" {
                method = Del
        }
        fmt.Fprintf(w, "\"method\": %q,\n\"url\": %q", method, uri)

        // data
        if data != "" && method != Get {
                fmt.Fprintf(w, ", \"body\": \n%q\n", data)
        }

        // Add cookie as header
        if c := BuildK6CookiesValues(cookies); c != "" {
                headers = append(headers, Header{Name: "Cookie", Value: c})
        }

        if header := BuildK6Headers(headers); len(header) > 0 {
                fmt.Fprintf(w, ", \"params\": {\n%s\n}\n", header)
        }

        fmt.Fprint(w, "}")
        if err := w.Flush(); err != nil {
                return "", err
        }

        // json indentation
        var buffer bytes.Buffer
        err := json.Indent(&buffer, b.Bytes(), "", "    ")
        if err != nil {
                return "", err
        }

        return buffer.String(), nil
}

// Build the string representation of a K6 headers object from a given HAR.NVP array
func BuildK6Headers(headers []Header) string {
        if len(headers) == 0 {
                return ""
        }
        m := make(map[string]Header)

        var h []string
        for _, header := range headers {
                if header.Name[0] != ':' { // avoid SPDY's colon headers
                        // avoid header duplicity
                        _, exists := m[strings.ToLower(header.Name)]
                        if !exists {
                                m[strings.ToLower(header.Name)] = header
                                h = append(h, fmt.Sprintf("%q : %q", header.Name, header.Value))
                        }
                }
        }

        return fmt.Sprintf("\"headers\" : { %v }", strings.Join(h, ", "))
}

// Build the string representation of K6 cookie values from a given HAR.Cookie array
func BuildK6CookiesValues(cookies []Cookie) string {
        if len(cookies) == 0 {
                return ""
        }

        var c []string
        for _, cookie := range cookies {
                c = append(c, fmt.Sprintf("%v=%v", cookie.Name, cookie.Value))
        }

        return strings.Join(c, "; ")
}

// Returns true if the given url is allowed from the only (only domains) and skip (skip domains) values, otherwise false
func IsAllowedURL(url string, only, skip []string) bool {
        if len(only) != 0 {
                for _, v := range only {
                        v = strings.Trim(v, " ")
                        if v != "" && strings.Contains(url, v) {
                                return true
                        }
                }
                return false
        }
        if len(skip) != 0 {
                for _, v := range skip {
                        v = strings.Trim(v, " ")
                        if v != "" && strings.Contains(url, v) {
                                return false
                        }
                }
        }
        return true
}

func WriteK6Script(w *bufio.Writer, h *HAR, includeCodeCheck bool, batchTime uint, only, skip []string, maxRequestsBatch uint) error {
        if includeCodeCheck {
                fmt.Fprint(w, "import { group, check, sleep } from 'k6';\n")
        } else {
                fmt.Fprint(w, "import { group, sleep } from 'k6';\n")
        }
        fmt.Fprint(w, "import http from 'k6/http';\n\n")

        fmt.Fprintf(w, "// Version: %v\n", h.Log.Version)
        fmt.Fprintf(w, "// Creator: %v\n", h.Log.Creator.Name)

        if h.Log.Browser != nil { // browser is optional
                fmt.Fprintf(w, "// Browser: %v\n", h.Log.Browser.Name)
        }
        if h.Log.Comment != "" {
                fmt.Fprintf(w, "// %v\n", h.Log.Comment)
        }

        fmt.Fprint(w, "\nexport default function() {\n\nlet res;\n\n")

        // name used by group entries
        pagenames := make(map[string]string)
        for _, e := range h.Log.Pages {
                pagenames[e.ID] = fmt.Sprintf("%s - %s", e.ID, e.Title)
        }

        // grouping by page and URL filtering
        groups := make(map[string][]*Entry)
        var nameGroups []string
        for _, e := range h.Log.Entries {

                // URL filtering
                u, err := url.Parse(e.Request.URL)
                if err != nil {
                        return err
                }
                if !IsAllowedURL(u.Host, only, skip) {
                        continue
                }

                // avoid multipart/form-data requests until k6 scripts can support binary data
                if e.Request.PostData != nil && strings.HasPrefix(e.Request.PostData.MimeType, "multipart/form-data") {
                        continue
                }

                // create new group o adding page to a existing one
                if _, ok := groups[e.Pageref]; !ok {
                        groups[e.Pageref] = append([]*Entry{}, e)
                        nameGroups = append(nameGroups, e.Pageref)
                } else {
                        groups[e.Pageref] = append(groups[e.Pageref], e)
                }
        }

        for _, n := range nameGroups {

                // sort entries by requests started date time
                sort.Sort(byRequestDate(groups[n]))

                fmt.Fprintf(w, "group(\"%v\", function() {\n", pagenames[n])

                if batchTime > 0 {
                        // batch mode, multiple HTTP requests together
                        entries := groupHarEntriesByIntervals(groups[n], groups[n][0].StartedDateTime, batchTime, maxRequestsBatch)

                        fmt.Fprint(w, "\tlet req\n")

                        for _, e := range entries {
                                var statuses []int

                                fmt.Fprint(w, "\treq = [\n")
                                for _, r := range e {
                                        var data string
                                        if r.Request.PostData != nil {
                                                data = r.Request.PostData.Text
                                        }
                                        b, err := BuildK6RequestObject(r.Request.Method, r.Request.URL, data, r.Request.Headers, r.Request.Cookies)
                                        if err != nil {
                                                return err
                                        }
                                        fmt.Fprintf(w, "%v,\n", b)
                                        statuses = append(statuses, r.Response.Status)
                                }
                                fmt.Fprint(w, "];\n")

                                fmt.Fprint(w, "\tres = http.batch(req);\n")
                                if includeCodeCheck {
                                        for i, s := range statuses {
                                                if s > 0 { // avoid empty responses, browsers with adblockers, antivirus.. can block HTTP requests
                                                        fmt.Fprintf(w, "\tcheck(res[%v], {\n\t\t\"status is %v\": (r) => r.status === %v,\n\t});\n", i, s, s)
                                                }
                                        }
                                }
                        }
                } else {
                        // no batch mode
                        for _, e := range groups[n] {
                                var data string
                                if e.Request.PostData != nil {
                                        data = e.Request.PostData.Text
                                }
                                b, err := BuildK6Request(e.Request.Method, e.Request.URL, data, e.Request.Headers, e.Request.Cookies)
                                if err != nil {
                                        return err
                                }
                                fmt.Fprintf(w, "res = %v", b)
                                if includeCodeCheck {
                                        if e.Response.Status > 0 { // avoid empty responses, browsers with adblockers, antivirus.. can block HTTP requests
                                                fmt.Fprintf(w, "check(res, {\n\"status is %v\": (r) => r.status === %v,\n});\n", e.Response.Status, e.Response.Status)
                                        }
                                }
                        }
                }

                // random sleep from 100ms to 500ms
                fmt.Fprintf(w, "\tsleep(%.1f);\n", float64(rand.Intn(500)+100)/1000)

                fmt.Fprint(w, "});\n")
        }

        fmt.Fprint(w, "\n}\n")
        err := w.Flush()

        return err
}

func groupHarEntriesByIntervals(entries []*Entry, starttime time.Time, interval uint, maxentries uint) [][]*Entry {
        var ordered [][]*Entry
        var j int

        if interval > 0 {
                t := starttime
                d := time.Duration(interval) * time.Millisecond

                for _, e := range entries {
                        // new interval by date
                        if e.StartedDateTime.Sub(t) >= d {
                                t = t.Add(d)
                                j++
                        }
                        if len(ordered) == j {
                                ordered = append(ordered, []*Entry{})
                        }
                        // new interval by maxentries value
                        if len(ordered[j]) == int(maxentries) {
                                ordered = append(ordered, []*Entry{})
                                j++
                        }
                        ordered[j] = append(ordered[j], e)
                }
        }

        return ordered
}
