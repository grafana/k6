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
	"fmt"
	"testing"
	"time"

	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
)

func TestBuildK6Cookies(t *testing.T) {
	var cookies = []struct {
		values   []Cookie
		expected string
	}{
		{[]Cookie{{Name: "a", Value: "b"}}, "a=b"},
		{[]Cookie{{Name: "a", Value: "b"}, {Name: "c", Value: "d"}}, "a=b; c=d"},
	}

	for _, pair := range cookies {
		v := BuildK6CookiesValues(pair.values)
		if v != pair.expected {
			t.Errorf("BuildK6Cookies(%v): expected %v, actual %v", pair.values, pair.expected, v)
		}
	}
}

func TestBuildK6Headers(t *testing.T) {
	var headers = []struct {
		values   []Header
		expected string
	}{
		{[]Header{{"name", "1"}, {"name", "2"}}, "\"headers\" : { \"name\" : \"1\" }"},
		{[]Header{{"name", "1"}, {"Name", "2"}}, "\"headers\" : { \"name\" : \"1\" }"},
		{[]Header{{"Name", "1"}, {"name", "2"}}, "\"headers\" : { \"Name\" : \"1\" }"},
		{[]Header{{"name", "value"}, {"name2", "value2"}}, "\"headers\" : { \"name\" : \"value\", \"name2\" : \"value2\" }"},
		{[]Header{{"accept-language", "es-ES,es;q=0.8"}}, "\"headers\" : { \"accept-language\" : \"es-ES,es;q=0.8\" }"},
		{[]Header{{":host", "localhost"}}, "\"headers\" : {  }"}, // avoid SPDY’s colon headers
	}

	for _, pair := range headers {
		v := BuildK6Headers(pair.values)
		if v != pair.expected {
			t.Errorf("BuildK6Headers(%v): expected %v, actual %v", pair.values, pair.expected, v)
		}
	}
}

func TestBuildK6Request(t *testing.T) {
	v, err := BuildK6Request("get", "http://www.google.es", "", []Header{{"accept-language", "es-ES,es;q=0.8"}}, []Cookie{{Name: "a", Value: "b"}})

	if err != nil {
		t.Error(err)
	}

	_, err = js.New(&lib.SourceData{
		Filename: "/script.js",
		Data:     []byte(fmt.Sprintf("export default function() { %v }", v)),
	}, afero.NewMemMapFs())

	if err != nil {
		t.Error(err)
	}
}

func TestBuildK6RequestObject(t *testing.T) {
	v, err := BuildK6RequestObject("get", "http://www.google.es", "", []Header{{"accept-language", "es-ES,es;q=0.8"}}, []Cookie{{Name: "a", Value: "b"}})
	if err != nil {
		t.Error(err)
	}
	_, err = js.New(&lib.SourceData{
		Filename: "/script.js",
		Data:     []byte(fmt.Sprintf("export default function() { res = http.batch([%v]); }", v)),
	}, afero.NewMemMapFs())

	if err != nil {
		t.Error(err)
	}
}

func TestIsAllowedURL(t *testing.T) {
	var allowed = []struct {
		url      string
		only     []string
		skip     []string
		expected bool
	}{
		{"http://www.google.com/", []string{}, []string{}, true},
		{"http://www.google.com/", []string{"google.com"}, []string{}, true},
		{"https://www.google.com/", []string{"google.com"}, []string{}, true},
		{"https://www.google.com/", []string{"http://"}, []string{}, false},
		{"http://www.google.com/?hl=en", []string{"http://www.google.com"}, []string{}, true},
		{"http://www.google.com/?hl=en", []string{"google.com", "google.co.uk"}, []string{}, true},
		{"http://www.google.com/?hl=en", []string{}, []string{"google.com"}, false},
		{"http://www.google.com/?hl=en", []string{}, []string{"google.co.uk"}, true},
	}

	for _, s := range allowed {
		v := IsAllowedURL(s.url, s.only, s.skip)
		if v != s.expected {
			t.Errorf("IsAllowedURL(%v, %v, %v): expected %v, actual %v", s.url, s.only, s.skip, s.expected, v)
		}
	}
}

func TestGroupHarEntriesByIntervals(t *testing.T) {
	// max number of requests in a batch statement
	const maxentries uint = 5

	t1 := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)

	entries := []*Entry{}

	// 10 time entries with increments of 100ms (from 0 a 1000ms)
	for i := 0; i < 10; i++ {
		entries = append(entries, &Entry{StartedDateTime: t1.Add(time.Duration(100*i) * time.Millisecond)})
	}

	splitValues := []struct {
		diff, groups uint
	}{
		{0, 0},
		{1000, 2},
		{100, 10},
		{500, 2},
		{800, 3}, // group with 5 entries, group with 3 entries, group with 2 entries
	}

	for _, v := range splitValues {
		result := groupHarEntriesByIntervals(entries, t1, v.diff, maxentries)
		if len(result) != int(v.groups) {
			t.Errorf("groupHarEntriesByIntervals(%v, %v, %v, %v) Expected %v, actual %v", entries, t1, v.diff, maxentries, v.groups, len(result))
		}
	}
}
