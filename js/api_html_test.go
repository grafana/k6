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

package js

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHTML(t *testing.T) {
	if testing.Short() {
		return
	}

	assert.NoError(t, runSnippet(`
	import { parseHTML } from "k6/html";
	let html = "This is a <span id='what'>test snippet</span>.";
	export default function() { parseHTML(html); }
	`))
}

func TestHTMLText(t *testing.T) {
	if testing.Short() {
		return
	}

	assert.NoError(t, runSnippet(`
	import { _assert } from "k6";
	import { parseHTML } from "k6/html";
	let html = "This is a <span id='what'>test snippet</span>.";
	export default function() {
		let doc = parseHTML(html);
		_assert(doc.text() === "This is a test snippet.");
	}
	`))
}

func TestHTMLFindText(t *testing.T) {
	if testing.Short() {
		return
	}

	assert.NoError(t, runSnippet(`
	import { _assert } from "k6";
	import { parseHTML } from "k6/html";
	let html = "This is a <span id='what'>test snippet</span>.";
	export default function() {
		let doc = parseHTML(html);
		_assert(doc.find('#what').text() === "test snippet");
	}
	`))
}

func TestHTMLAddSelector(t *testing.T) {
	if testing.Short() {
		return
	}

	assert.NoError(t, runSnippet(`
	import { _assert } from "k6";
	import { parseHTML } from "k6/html";
	let html = "<span id='sub'>This</span> is a <span id='obj'>test snippet</span>.";
	export default function() {
		let doc = parseHTML(html);
		_assert(doc.find('#sub').add('#obj').text() === "Thistest snippet");
	}
	`))
}

func TestHTMLAddSelection(t *testing.T) {
	if testing.Short() {
		return
	}

	assert.NoError(t, runSnippet(`
	import { _assert } from "k6";
	import { parseHTML } from "k6/html";
	let html = "<span id='sub'>This</span> is a <span id='obj'>test snippet</span>.";
	export default function() {
		let doc = parseHTML(html);
		_assert(doc.find('#sub').add(doc.find('#obj')).text() === "Thistest snippet");
	}
	`))
}
