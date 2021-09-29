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

package browsertest

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/k6io/xk6-browser/api"
	"github.com/oxtoacart/bpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/compiler"
	"go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/stats"
	"gopkg.in/guregu/null.v3"
)

func AttachFrame(bt *BrowserTest, page api.Page, frameID string, url string) api.Frame {
	pageFn := `async (frameId, url) => {
	    const frame = document.createElement('iframe');
	    frame.src = url;
	    frame.id = frameId;
	    document.body.appendChild(frame);
	    await new Promise(x => frame.onload = x);
	    return frame;
	}`
	handle := page.EvaluateHandle(bt.Runtime.ToValue(pageFn), bt.Runtime.ToValue(frameID), bt.Runtime.ToValue(url))
	return handle.AsElement().ContentFrame()
}

func DetachFrame(bt *BrowserTest, page api.Page, frameID string) {
	pageFn := `frameId => {
        document.getElementById(frameId).remove();
    }`
	page.Evaluate(bt.Runtime.ToValue(pageFn), bt.Runtime.ToValue(frameID))
}

func NewRuntime(
	t testing.TB,
) (*httpmultibin.HTTPMultiBin, *lib.State, chan stats.SampleContainer, *goja.Runtime, *context.Context) {
	tb := httpmultibin.NewHTTPMultiBin(t)

	root, err := lib.NewGroup("", nil)
	require.NoError(t, err)

	logger := logrus.New()
	logger.Level = logrus.DebugLevel

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	options := lib.Options{
		MaxRedirects: null.IntFrom(10),
		UserAgent:    null.StringFrom("TestUserAgent"),
		Throw:        null.BoolFrom(true),
		SystemTags:   &stats.DefaultSystemTagSet,
		Batch:        null.IntFrom(20),
		BatchPerHost: null.IntFrom(20),
		// HTTPDebug:    null.StringFrom("full"),
	}
	samples := make(chan stats.SampleContainer, 1000)

	state := &lib.State{
		Options:   options,
		Logger:    logger,
		Group:     root,
		TLSConfig: tb.TLSClientConfig,
		Transport: tb.HTTPTransport,
		BPool:     bpool.NewBufferPool(1),
		Samples:   samples,
		Tags:      map[string]string{"group": root.Path},
	}

	ctx := new(context.Context)
	*ctx = lib.WithState(tb.Context, state)
	*ctx = common.WithRuntime(*ctx, rt)
	rt.Set("http", common.Bind(rt, new(http.GlobalHTTP).NewModuleInstancePerVU(), ctx))

	return tb, state, samples, rt, ctx
}

// runES6String Runs an ES6 string in the given runtime. Use this rather than writing ES5 in tests.
func RunES6String(tb testing.TB, rt *goja.Runtime, src string) (goja.Value, error) {
	var err error
	c := compiler.New(testutils.NewLogger(tb)) // TODO drop it ? maybe we will drop babel and this will be less needed
	src, _, err = c.Transform(src, "__string__")
	if err != nil {
		return goja.Undefined(), err
	}

	return rt.RunString(src)
}
