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
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/testutils"
	"github.com/oxtoacart/bpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	k6common "go.k6.io/k6/js/common"
	k6http "go.k6.io/k6/js/modules/k6/http"
	k6lib "go.k6.io/k6/lib"
	k6metrics "go.k6.io/k6/lib/metrics"
	k6test "go.k6.io/k6/lib/testutils/httpmultibin"
	k6stats "go.k6.io/k6/stats"
	"gopkg.in/guregu/null.v3"
)

type BrowserTest struct {
	Ctx          context.Context
	Runtime      *goja.Runtime
	State        *k6lib.State
	HTTPMultiBin *k6test.HTTPMultiBin
	Samples      chan k6stats.SampleContainer
	Browser      api.Browser
}

func NewBrowserTest(t testing.TB) *BrowserTest {
	tb := k6test.NewHTTPMultiBin(t)

	root, err := k6lib.NewGroup("", nil)
	require.NoError(t, err)

	logger := logrus.StandardLogger()

	rt := goja.New()
	rt.SetFieldNameMapper(k6common.FieldNameMapper{})

	options := k6lib.Options{
		MaxRedirects: null.IntFrom(10),
		UserAgent:    null.StringFrom("TestUserAgent"),
		Throw:        null.BoolFrom(true),
		SystemTags:   &k6stats.DefaultSystemTagSet,
		Batch:        null.IntFrom(20),
		BatchPerHost: null.IntFrom(20),
		// HTTPDebug:    null.StringFrom("full"),
	}
	samples := make(chan k6stats.SampleContainer, 1000)

	registry := k6metrics.NewRegistry()
	state := &k6lib.State{
		Options:        options,
		Logger:         logger,
		Group:          root,
		TLSConfig:      tb.TLSClientConfig,
		Transport:      tb.HTTPTransport,
		BPool:          bpool.NewBufferPool(1),
		Samples:        samples,
		Tags:           k6lib.NewTagMap(map[string]string{"group": root.Path}),
		BuiltinMetrics: k6metrics.RegisterBuiltinMetrics(registry),
	}

	ctx := new(context.Context)
	*ctx = k6lib.WithState(tb.Context, state)
	*ctx = k6common.WithRuntime(*ctx, rt)
	err = rt.Set("http", k6common.Bind(rt, new(k6http.GlobalHTTP).NewModuleInstancePerVU(), ctx))
	require.NoError(t, err)

	bt := chromium.NewBrowserType(*ctx).(*chromium.BrowserType)
	debug := false
	headless := true
	if v, found := os.LookupEnv("XK6_BROWSER_TEST_DEBUG"); found {
		debug, _ = strconv.ParseBool(v)
	}
	if v, found := os.LookupEnv("XK6_BROWSER_TEST_HEADLESS"); found {
		headless, _ = strconv.ParseBool(v)
	}
	launchOpts := rt.ToValue(struct {
		Debug    bool   `js:"debug"`
		Headless bool   `js:"headless"`
		SlowMo   string `js:"slowMo"`
		Timeout  string `js:"timeout"`
	}{
		Debug:    debug,
		Headless: headless,
		SlowMo:   "0s",
		Timeout:  "30s",
	})
	browser := bt.Launch(launchOpts)

	return &BrowserTest{
		Ctx:          bt.Ctx, // This context has the additional wrapping of common.WithLaunchOptions
		Runtime:      rt,
		State:        state,
		Browser:      browser,
		HTTPMultiBin: tb,
		Samples:      samples,
	}
}

func (bt *BrowserTest) LogHook() *testutils.SimpleLogrusHook {
	logHook := &testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.DebugLevel}}
	bt.State.Logger.SetLevel(logrus.DebugLevel)
	bt.State.Logger.AddHook(logHook)
	bt.State.Logger.SetOutput(ioutil.Discard)
	return logHook
}
