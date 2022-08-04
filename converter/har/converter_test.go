package har

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/metrics"
)

func TestBuildK6Headers(t *testing.T) {
	headers := []struct {
		values   []Header
		expected []string
	}{
		{[]Header{{"name", "1"}, {"name", "2"}}, []string{`"name": "1"`}},
		{[]Header{{"name", "1"}, {"name2", "2"}}, []string{`"name": "1"`, `"name2": "2"`}},
		{[]Header{{":host", "localhost"}}, []string{}},
	}

	for _, pair := range headers {
		v := buildK6Headers(pair.values)
		assert.Equal(t, len(v), len(pair.expected), fmt.Sprintf("params: %v", pair.values))
	}
}

func TestBuildK6RequestObject(t *testing.T) {
	req := &Request{
		Method:  "get",
		URL:     "http://www.google.es",
		Headers: []Header{{"accept-language", "es-ES,es;q=0.8"}},
		Cookies: []Cookie{{Name: "a", Value: "b"}},
	}
	v, err := buildK6RequestObject(req)
	assert.NoError(t, err)
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	_, err = js.New(
		&lib.TestPreInitState{
			Logger:         testutils.NewLogger(t),
			BuiltinMetrics: builtinMetrics,
			Registry:       registry,
		}, &loader.SourceData{
			URL:  &url.URL{Path: "/script.js"},
			Data: []byte(fmt.Sprintf("export default function() { res = http.batch([%v]); }", v)),
		}, nil)
	assert.NoError(t, err)
}

func TestBuildK6Body(t *testing.T) {
	bodyText := "ccustemail=ppcano%40gmail.com&size=medium&topping=cheese&delivery=12%3A00&comments="

	req := &Request{
		Method: "post",
		URL:    "http://www.google.es",
		PostData: &PostData{
			MimeType: "application/x-www-form-urlencoded",
			Text:     bodyText,
		},
	}
	postParams, plainText, err := buildK6Body(req)
	assert.NoError(t, err)
	assert.Equal(t, len(postParams), 0, "postParams should be empty")
	assert.Equal(t, bodyText, plainText)

	email := "user@mail.es"
	expectedEmailParam := fmt.Sprintf(`"email": %q`, email)

	req = &Request{
		Method: "post",
		URL:    "http://www.google.es",
		PostData: &PostData{
			MimeType: "application/x-www-form-urlencoded",
			Params: []Param{
				{Name: "email", Value: url.QueryEscape(email)},
				{Name: "pw", Value: "hola"},
			},
		},
	}
	postParams, plainText, err = buildK6Body(req)
	assert.NoError(t, err)
	assert.Equal(t, plainText, "", "expected empty plainText")
	assert.Equal(t, len(postParams), 2, "postParams should have two items")
	assert.Equal(t, postParams[0], expectedEmailParam, "expected unescaped value")
}
