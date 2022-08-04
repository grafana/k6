package js

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/metrics"
)

func TestNewSharedArrayIntegration(t *testing.T) {
	t.Parallel()
	data := `'use strict';
var SharedArray = require("k6/data").SharedArray;
function generateArray() {
    console.log("once");
    var n = 50;
    var arr = new Array(n);
    for (var i = 0 ; i <n; i++) {
        arr[i] = {value: "something" +i};
    }
    return arr;
}

var s = new SharedArray("something", generateArray);

exports.default = function() {
	if (s[2].value !== "something2") {
		throw new Error("bad s[2]="+s[2].value);
	}
	if (s.length != 50) {
		throw new Error("bad length " +_s.length);
	}

	var i = 0;
	for (var v of s) {
		if (v.value !== "something"+i) {
			throw new Error("bad v.value="+v.value+" for i="+i);
		}
		i++;
	}
}`

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Out = ioutil.Discard
	hook := testutils.SimpleLogrusHook{
		HookedLevels: []logrus.Level{logrus.InfoLevel, logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel},
	}
	logger.AddHook(&hook)

	r1, err := getSimpleRunner(t, "/script.js", data, logger)
	require.NoError(t, err)
	entries := hook.Drain()
	require.Len(t, entries, 1)
	assert.Equal(t, logrus.InfoLevel, entries[0].Level)
	assert.Equal(t, "once", entries[0].Message)

	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	r2, err := NewFromArchive(
		&lib.TestPreInitState{
			Logger:         logger,
			BuiltinMetrics: builtinMetrics,
			Registry:       registry,
		}, r1.MakeArchive())
	require.NoError(t, err)
	entries = hook.Drain()
	require.Len(t, entries, 1)
	assert.Equal(t, logrus.InfoLevel, entries[0].Level)
	assert.Equal(t, "once", entries[0].Message)

	testdata := map[string]*Runner{"Source": r1, "Archive": r2}
	for name, r := range testdata {
		r := r
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			samples := make(chan metrics.SampleContainer, 100)
			initVU, err := r.NewVU(1, 1, samples)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
			err = vu.RunOnce()
			require.NoError(t, err)
			entries := hook.Drain()
			assert.Len(t, entries, 0)
		})
	}
}
