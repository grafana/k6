/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
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

package tests

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestElementHandleWaitForSelector(t *testing.T) {
	t.Parallel()

	tb := TestBrowser(t)
	p := tb.NewPage(nil)
	t.Cleanup(func() {
		p.Close(nil)
	})

	p.SetContent(`<div class="root"></div>`, nil)

	root := p.Query(".root")
	p.Evaluate(tb.Runtime.ToValue(`
        () => {
		setTimeout(() => {
			const div = document.createElement('div');
			div.className = 'element-to-appear';
			div.appendChild(document.createTextNode("Hello World"));
			root = document.querySelector('.root');
			root.appendChild(div);
			}, 100);
		}
	`))
	element := root.WaitForSelector(".element-to-appear", tb.Runtime.ToValue(struct {
		Timeout int64 `js:"timeout"`
	}{Timeout: 1000}))

	require.NotNil(t, element, "expected element to have been found after wait")

	element.Dispose()
}
