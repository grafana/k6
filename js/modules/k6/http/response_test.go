/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
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
	w.WriteHeader(200)
	_, _ = w.Write(body)
}

func jsonHandler(w http.ResponseWriter, r *http.Request) {
	body := []byte(jsonData)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(200)
	_, _ = w.Write(body)
}

func TestResponse(t *testing.T) {
	tb, state, samples, rt, _ := newRuntime(t)
	defer tb.Cleanup()
	root := state.Group
	sr := tb.Replacer.Replace

	tb.Mux.HandleFunc("/myforms/get", myFormHandler)
	tb.Mux.HandleFunc("/json", jsonHandler)

	t.Run("Html", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			let res = http.request("GET", "HTTPBIN_URL/html");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			if (res.body.indexOf("Herman Melville - Moby-Dick") == -1) { throw new Error("wrong body: " + res.body); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/html"), "", 200, "")

		t.Run("html", func(t *testing.T) {
			_, err := common.RunString(rt, `
				if (res.html().find("h1").text() != "Herman Melville - Moby-Dick") { throw new Error("wrong title: " + res.body); }
			`)
			assert.NoError(t, err)

			t.Run("shorthand", func(t *testing.T) {
				_, err := common.RunString(rt, `
					if (res.html("h1").text() != "Herman Melville - Moby-Dick") { throw new Error("wrong title: " + res.body); }
				`)
				assert.NoError(t, err)
			})

			t.Run("url", func(t *testing.T) {
				_, err := common.RunString(rt, sr(`
					if (res.html().url != "HTTPBIN_URL/html") { throw new Error("url incorrect: " + res.html().url); }
				`))
				assert.NoError(t, err)
			})
		})

		t.Run("group", func(t *testing.T) {
			g, err := root.Group("my group")
			if assert.NoError(t, err) {
				old := state.Group
				state.Group = g
				defer func() { state.Group = old }()
			}

			_, err = common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/html");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				if (res.body.indexOf("Herman Melville - Moby-Dick") == -1) { throw new Error("wrong body: " + res.body); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/html"), "", 200, "::my group")
		})
	})
	t.Run("Json", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			let res = http.request("GET", "HTTPBIN_URL/get?a=1&b=2");
			if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			if (res.json().args.a != "1") { throw new Error("wrong ?a: " + res.json().args.a); }
			if (res.json().args.b != "2") { throw new Error("wrong ?b: " + res.json().args.b); }
		`))
		assert.NoError(t, err)
		assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/get?a=1&b=2"), "", 200, "")

		t.Run("Invalid", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`http.request("GET", "HTTPBIN_URL/html").json();`))
			assert.EqualError(t, err, "GoError: invalid character '<' looking for beginning of value\n Invalid character at offset 1\n <<--(Invalid Character)!DOCTYPE html>\n<html>\n  <head>\n  </head>\n  <body>\n      <h1>Herman Melville - Moby-Dick</h1>\n\n      <div>\n        <p>\n          Availing himself of the mild, summer-cool weather that now reigned in these latitudes, and in preparation for the peculiarly active pursuits shortly to be anticipated, Perth, the begrimed, blistered old blacksmith, had not removed his portable forge to the hold again, after concluding his contributory work for Ahab's leg, but still retained it on deck, fast lashed to ringbolts by the foremast; being now almost incessantly invoked by the headsmen, and harpooneers, and bowsmen to do some little job for them; altering, or repairing, or new shaping their various weapons and boat furniture. Often he would be surrounded by an eager circle, all waiting to be served; holding boat-spades, pike-heads, harpoons, and lances, and jealously watching his every sooty movement, as he toiled. Nevertheless, this old man's was a patient hammer wielded by a patient arm. No murmur, no impatience, no petulance did come from him. Silent, slow, and solemn; bowing over still further his chronically broken back, he toiled away, as if toil were life itself, and the heavy beating of his hammer the heavy beating of his heart. And so it was.—Most miserable! A peculiar walk in this old man, a certain slight but painful appearing yawing in his gait, had at an early period of the voyage excited the curiosity of the mariners. And to the importunity of their persisted questionings he had finally given in; and so it came to pass that every one now knew the shameful story of his wretched fate. Belated, and not innocently, one bitter winter's midnight, on the road running between two country towns, the blacksmith half-stupidly felt the deadly numbness stealing over him, and sought refuge in a leaning, dilapidated barn. The issue was, the loss of the extremities of both feet. Out of this revelation, part by part, at last came out the four acts of the gladness, and the one long, and as yet uncatastrophied fifth act of the grief of his life's drama. He was an old man, who, at the age of nearly sixty, had postponedly encountered that thing in sorrow's technicals called ruin. He had been an artisan of famed excellence, and with plenty to do; owned a house and garden; embraced a youthful, daughter-like, loving wife, and three blithe, ruddy children; every Sunday went to a cheerful-looking church, planted in a grove. But one night, under cover of darkness, and further concealed in a most cunning disguisement, a desperate burglar slid into his happy home, and robbed them all of everything. And darker yet to tell, the blacksmith himself did ignorantly conduct this burglar into his family's heart. It was the Bottle Conjuror! Upon the opening of that fatal cork, forth flew the fiend, and shrivelled up his home. Now, for prudent, most wise, and economic reasons, the blacksmith's shop was in the basement of his dwelling, but with a separate entrance to it; so that always had the young and loving healthy wife listened with no unhappy nervousness, but with vigorous pleasure, to the stout ringing of her young-armed old husband's hammer; whose reverberations, muffled by passing through the floors and walls, came up to her, not unsweetly, in her nursery; and so, to stout Labor's iron lullaby, the blacksmith's infants were rocked to slumber. Oh, woe on woe! Oh, Death, why canst thou not sometimes be timely? Hadst thou taken this old blacksmith to thyself ere his full ruin came upon him, then had the young widow had a delicious grief, and her orphans a truly venerable, legendary sire to dream of in their after years; and all of them a care-killing competency.\n        </p>\n      </div>\n  </body>\n</html>\n")
		})
	})
	t.Run("JsonSelector", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			let res = http.request("GET", "HTTPBIN_URL/json");
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
		assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/json"), "", 200, "")
	})

	t.Run("SubmitForm", func(t *testing.T) {
		t.Run("withoutArgs", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/forms/post");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.submitForm()
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				let data = res.json().form
				if (data.custname[0] !== "" ||
					data.extradata !== undefined ||
					data.comments[0] !== "" ||
					data.custemail[0] !== "" ||
					data.custtel[0] !== "" ||
					data.delivery[0] !== ""
				) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), "", 200, "")
		})

		t.Run("withFields", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/forms/post");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.submitForm({ fields: { custname: "test", extradata: "test2" } })
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				let data = res.json().form
				if (data.custname[0] !== "test" ||
					data.extradata[0] !== "test2" ||
					data.comments[0] !== "" ||
					data.custemail[0] !== "" ||
					data.custtel[0] !== "" ||
					data.delivery[0] !== ""
				) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), "", 200, "")
		})

		t.Run("withRequestParams", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/forms/post");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.submitForm({ params: { headers: { "My-Fancy-Header": "SomeValue" } }})
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				let headers = res.json().headers
				if (headers["My-Fancy-Header"][0] !== "SomeValue" ) { throw new Error("incorrect headers: " + JSON.stringify(headers)); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), "", 200, "")
		})

		t.Run("withFormSelector", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/forms/post");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.submitForm({ formSelector: 'form[method="post"]' })
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				let data = res.json().form
				if (data.custname[0] !== "" ||
					data.extradata !== undefined ||
					data.comments[0] !== "" ||
					data.custemail[0] !== "" ||
					data.custtel[0] !== "" ||
					data.delivery[0] !== ""
				) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "POST", sr("HTTPBIN_URL/post"), "", 200, "")
		})

		t.Run("withNonExistentForm", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/forms/post");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res.submitForm({ formSelector: "#doesNotExist" })
			`))
			assert.EqualError(t, err, sr("GoError: no form found for selector '#doesNotExist' in response 'HTTPBIN_URL/forms/post'"))
		})

		t.Run("withGetMethod", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/myforms/get");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.submitForm()
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				let data = res.json().query
				if (data.input_with_value[0] !== "value" ||
					data.input_without_value[0] !== "" ||
					data.select_one[0] !== "yes this option" ||
					data.select_multi[0] !== "option 2,option 3" ||
					data.textarea[0] !== "Lorem ipsum dolor sit amet"
				) { throw new Error("incorrect body: " + JSON.stringify(data, null, 4) ); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/myforms/get"), "", 200, "")
		})
	})

	t.Run("ClickLink", func(t *testing.T) {
		t.Run("withoutArgs", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/links/10/0");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.clickLink()
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/links/10/1"), "", 200, "")
		})

		t.Run("withSelector", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/links/10/0");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.clickLink({ selector: 'a:nth-child(4)' })
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/links/10/4"), "", 200, "")
		})

		t.Run("withNonExistentLink", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL/links/10/0");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.clickLink({ selector: 'a#doesNotExist' })
			`))
			assert.EqualError(t, err, sr("GoError: no element found for selector 'a#doesNotExist' in response 'HTTPBIN_URL/links/10/0'"))
		})

		t.Run("withRequestParams", func(t *testing.T) {
			_, err := common.RunString(rt, sr(`
				let res = http.request("GET", "HTTPBIN_URL");
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				res = res.clickLink({ selector: 'a[href="/get"]', params: { headers: { "My-Fancy-Header": "SomeValue" } } })
				if (res.status != 200) { throw new Error("wrong status: " + res.status); }
				let headers = res.json().headers
				if (headers["My-Fancy-Header"][0] !== "SomeValue" ) { throw new Error("incorrect headers: " + JSON.stringify(headers)); }
			`))
			assert.NoError(t, err)
			assertRequestMetricsEmitted(t, stats.GetBufferedSamples(samples), "GET", sr("HTTPBIN_URL/get"), "", 200, "")
		})
	})
}

func BenchmarkResponseJson(b *testing.B) {
	ctx := context.Background()
	rt := goja.New()
	ctx = common.WithRuntime(ctx, rt)
	testCases := []struct {
		selector string
	}{
		{"glossary.GlossDiv.GlossList.GlossEntry.title"},
		{"glossary.GlossDiv.GlossList.GlossEntry.int"},
		{"glossary.GlossDiv.GlossList.GlossEntry.intArray"},
		{"glossary.GlossDiv.GlossList.GlossEntry.mixedArray"},
		{"glossary.friends"},
		{"glossary.friends.#.first"},
		{"glossary.GlossDiv.GlossList.GlossEntry.GlossDef"},
		{"glossary"},
	}
	for _, tc := range testCases {
		b.Run(fmt.Sprintf("Selector %s ", tc.selector), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				resp := &HTTPResponse{ctx: ctx, Body: jsonData}
				resp.Json(tc.selector)
			}
		})
	}

	b.Run("Without selector", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			resp := &HTTPResponse{ctx: ctx, Body: jsonData}
			resp.Json()
		}
	})
}
