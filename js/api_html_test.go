package js

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseHTML(t *testing.T) {
	assert.NoError(t, runSnippet(`
	import { parseHTML } from "speedboat/html";
	let html = "This is a <span id='what'>test snippet</span>.";
	export default function() { parseHTML(html); }
	`))
}

func TestHTMLText(t *testing.T) {
	assert.NoError(t, runSnippet(`
	import { _assert } from "speedboat";
	import { parseHTML } from "speedboat/html";
	let html = "This is a <span id='what'>test snippet</span>.";
	export default function() {
		let doc = parseHTML(html);
		_assert(doc.text() === "This is a test snippet.");
	}
	`))
}

func TestHTMLFindText(t *testing.T) {
	assert.NoError(t, runSnippet(`
	import { _assert } from "speedboat";
	import { parseHTML } from "speedboat/html";
	let html = "This is a <span id='what'>test snippet</span>.";
	export default function() {
		let doc = parseHTML(html);
		_assert(doc.find('#what').text() === "test snippet");
	}
	`))
}
