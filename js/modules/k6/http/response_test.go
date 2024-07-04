package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

const testGetFormHTML = `
<html>
<head>
	<title>This is the title</title>
</head>
<body>
	<form method="get" id="form1">
		<input name="input_with_value" type="text" value="value"/>
		<input name="input_without_value" type="text"/>
		<select name="select_one">
			<option value="not this option">no</option>
			<option value="yes this option" selected>yes</option>
		</select>
		<select name="select_multi" multiple>
			<option>option 1</option>
			<option selected>option 2</option>
			<option selected>option 3</option>
		</select>
		<textarea name="textarea" multiple>Lorem ipsum dolor sit amet</textarea>
	</form>
</body>
`

const jsonData = `{"glossary": {
    "friends": [
      {"first": "Dale", "last": "Murphy", "age": 44},
      {"first": "Roger", "last": "Craig", "age": 68},
      {"first": "Jane", "last": "Murphy", "age": 47}],
	"GlossDiv": {
	  "title": "S",
	  "GlossList": {
	    "GlossEntry": {
	      "ID": "SGML",
	      "SortAs": "SGML",
	      "GlossTerm": "Standard Generalized Markup Language",
	      "Acronym": "SGML",
	      "Abbrev": "ISO 8879:1986",
	      "GlossDef": {
            "int": 1123456,
            "null": null,
            "intArray": [1,2,3],
            "mixedArray": ["123",123,true,null],
            "boolean": true,
            "title": "example glossary",
            "para": "A meta-markup language, used to create markup languages such as DocBook.",
	  "GlossSeeAlso": ["GML","XML"]},
	"GlossSee": "markup"}}}}}`

const invalidJSONData = `{
	"a":"apple",
	"t":testing"
}`

func myFormHandler(w http.ResponseWriter, r *http.Request) {
	var body []byte
	var err error
	if r.URL.RawQuery != "" {
		body, err = json.Marshal(struct {
			Query url.Values `json:"query"`
		}{
			Query: r.URL.Query(),
		})
		if err != nil {
			body = []byte(`{"error": "failed serializing json"}`)
		}
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "text/html")
		body = []byte(testGetFormHTML)
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func jsonHandler(w http.ResponseWriter, _ *http.Request) {
	body := []byte(jsonData)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func invalidJSONHandler(w http.ResponseWriter, _ *http.Request) {
	body := []byte(invalidJSONData)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func TestResponse(t *testing.T) {
	ts := newTestCase(t)
	tb := ts.tb
	samples := ts.samples
	rt := ts.runtime.VU.Runtime()
	state := ts.runtime.VU.State()
	sr := tb.Replacer.Replace

	tb.Mux.HandleFunc("/myforms/get", myFormHandler)
	tb.Mux.HandleFunc("/json", jsonHandler)
	tb.Mux.HandleFunc("/invalidjson", invalidJSONHandler)

	t.Run("Html", func(t *testing.T) {
		_, err := rt.RunString(sr(`
			var res = http.request("GET", "HTTPBIN_URL/html");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			if (res.body.indexOf("Herman Melville - Moby-Dick") == -1) { throw new Error("wrong body: " + res.body); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/html"), 200, "")

		t.Run("html", func(t *testing.T) {
			_, err := rt.RunString(`
				if (res.html().find("h1").text() != "Herman Melville - Moby-Dick") { throw new Error("wrong title: " + res.body); }
			`)
			assert.NoError(t, err)

			t.Run("shorthand", func(t *testing.T) {
				_, err := rt.RunString(`
					if (res.html("h1").text() != "Herman Melville - Moby-Dick") { throw new Error("wrong title: " + res.body); }
				`)
				assert.NoError(t, err)
			})

			t.Run("url", func(t *testing.T) {
				_, err := rt.RunString(sr(`
					if (res.html().url != "HTTPBIN_URL/html") { throw new Error("url incorrect: " + res.html().url); }
				`))
				assert.NoError(t, err)
			})
		})

		t.Run("group", func(t *testing.T) {
			groupName, err := lib.NewGroupPath(lib.RootGroupPath, "my group")
			require.NoError(t, err)
			state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
				tagsAndMeta.SetTag("group", groupName)
			})
			defer func() {
				state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
					tagsAndMeta.SetTag("group", "")
				})
			}()

			_, err = rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/html");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.body.indexOf("Herman Melville - Moby-Dick") == -1) { throw new Error("wrong body: " + res.body); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/html"), 200, "::my group")
		})

		t.Run("NoResponseBody", func(t *testing.T) {
			_, err := rt.RunString(sr(`http.get("HTTPBIN_URL/html", {responseType: 'none'}).html();`))
			require.NotNil(t, err)
			assert.Contains(t, err.Error(), "the body is null so we can't transform it to HTML"+
				" - this likely was because of a request error getting the response")
		})
	})
	t.Run("Json", func(t *testing.T) {
		_, err := rt.RunString(sr(`
			var res = http.request("GET", "HTTPBIN_URL/get?a=1&b=2");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			if (res.json().args.a != "1") { throw new Error("wrong ?a: " + res.json().args.a); }
			if (res.json().args.b != "2") { throw new Error("wrong ?b: " + res.json().args.b); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/get?a=1&b=2"), 200, "")

		t.Run("Invalid", func(t *testing.T) {
			_, err := rt.RunString(sr(`http.request("GET", "HTTPBIN_URL/html").json();`))
			assert.Contains(t, err.Error(), "cannot parse json due to an error at line 1, character 2 , error: invalid character '<' looking for beginning of value")
		})

		t.Run("Invalid", func(t *testing.T) {
			_, err := rt.RunString(sr(`http.request("GET", "HTTPBIN_URL/invalidjson").json();`))
			assert.Contains(t, err.Error(), "cannot parse json due to an error at line 3, character 9 , error: invalid character 'e' in literal true (expecting 'r')")
		})

		t.Run("NoResponseBody", func(t *testing.T) {
			_, err := rt.RunString(sr(`http.get("HTTPBIN_URL/json", {responseType: 'none'}).json();`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "the body is null so we can't transform it to JSON"+
				" - this likely was because of a request error getting the response")
		})
	})
	t.Run("JsonSelector", func(t *testing.T) {
		_, err := rt.RunString(sr(`
			var res = http.request("GET", "HTTPBIN_URL/json");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }

			var value = res.json("glossary.friends.1")
	        if (typeof value != "object")
				{ throw new Error("wrong type of result value: " + value); }
	        if (value["first"] != "Roger")
				{ throw new Error("Expected Roger for key first but got: " + value["first"]); }

			value = res.json("glossary.int1")
	        if (value != undefined)
				{ throw new Error("Expected undefined, but got: " + value); }

			value = res.json("glossary.null")
	        if (value != null)
				{ throw new Error("Expected null, but got: " + value); }

			value = res.json("glossary.GlossDiv.GlossList.GlossEntry.GlossDef.intArray.#")
	        if (value != 3)
				{ throw new Error("Expected num 3, but got: " + value); }

			value = res.json("glossary.GlossDiv.GlossList.GlossEntry.GlossDef.intArray")[2]
	        if (value != 3)
 				{ throw new Error("Expected, num 3, but got: " + value); }

			value = res.json("glossary.GlossDiv.GlossList.GlossEntry.GlossDef.boolean")
	        if (value != true)
				{ throw new Error("Expected boolean true, but got: " + value); }

			value = res.json("glossary.GlossDiv.GlossList.GlossEntry.GlossDef.title")
	        if (value != "example glossary")
				{ throw new Error("Expected 'example glossary'', but got: " + value); }

			value =	res.json("glossary.friends.#.first")[0]
	        if (value != "Dale")
				{ throw new Error("Expected 'Dale', but got: " + value); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/json"), 200, "")
	})

	t.Run("SubmitForm", func(t *testing.T) {
		t.Run("withoutArgs", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/forms/post");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.submitForm()
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				var data = res.json().form
				if (data.custname[0] !== "" ||
					data.extradata !== undefined ||
					data.comments[0] !== "" ||
					data.custemail[0] !== "" ||
					data.custtel[0] !== "" ||
					data.delivery[0] !== ""
				) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), 200, "")
		})

		t.Run("withFields", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/forms/post");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.submitForm({ fields: { custname: "test", extradata: "test2" } })
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				var data = res.json().form
				if (data.custname[0] !== "test" ||
					data.extradata[0] !== "test2" ||
					data.comments[0] !== "" ||
					data.custemail[0] !== "" ||
					data.custtel[0] !== "" ||
					data.delivery[0] !== ""
				) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), 200, "")
		})

		t.Run("withRequestParams", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/forms/post");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.submitForm({ params: { headers: { "My-Fancy-Header": "SomeValue" } }})
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				var headers = res.json().headers
				if (headers["My-Fancy-Header"][0] !== "SomeValue" ) { throw new Error("incorrect headers: " + JSON.stringify(headers)); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), 200, "")
		})

		t.Run("withFormSelector", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/forms/post");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.submitForm({ formSelector: 'form[method="post"]' })
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				var data = res.json().form
				if (data.custname[0] !== "" ||
					data.extradata !== undefined ||
					data.comments[0] !== "" ||
					data.custemail[0] !== "" ||
					data.custtel[0] !== "" ||
					data.delivery[0] !== ""
				) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), 200, "")
		})

		t.Run("withNonExistentForm", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/forms/post");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res.submitForm({ formSelector: "#doesNotExist" })
			`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), sr("no form found for selector '#doesNotExist' in response 'HTTPBIN_URL/forms/post'"))
		})

		t.Run("withGetMethod", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/myforms/get");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.submitForm()
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				var data = res.json().query
				if (data.input_with_value[0] !== "value" ||
					data.input_without_value[0] !== "" ||
					data.select_one[0] !== "yes this option" ||
					data.select_multi[0] !== "option 2,option 3" ||
					data.textarea[0] !== "Lorem ipsum dolor sit amet"
				) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
			`))
			require.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/myforms/get"), 200, "")
		})
	})

	t.Run("ClickLink", func(t *testing.T) {
		t.Run("withoutArgs", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/links/10/0");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.clickLink()
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/links/10/1"), 200, "")
		})

		t.Run("withSelector", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/links/10/0");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.clickLink({ selector: 'a:nth-child(4)' })
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/links/10/4"), 200, "")
		})

		t.Run("withNonExistentLink", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL/links/10/0");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.clickLink({ selector: 'a#doesNotExist' })
			`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), sr("no element found for selector 'a#doesNotExist' in response 'HTTPBIN_URL/links/10/0'"))
		})

		t.Run("withRequestParams", func(t *testing.T) {
			_, err := rt.RunString(sr(`
				var res = http.request("GET", "HTTPBIN_URL");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.clickLink({ selector: 'a[href="/get"]', params: { headers: { "My-Fancy-Header": "SomeValue" } } })
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				var headers = res.json().headers
				if (headers["My-Fancy-Header"][0] !== "SomeValue" ) { throw new Error("incorrect headers: " + JSON.stringify(headers)); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, metrics.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/get"), 200, "")
		})
	})
}
