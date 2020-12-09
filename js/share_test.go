/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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
	"context"
	"io/ioutil"
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/stats"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitContextNewShareable(t *testing.T) {
	data := `'use strict';
function generateArray() {
    console.log("once");
    var n = 50;
    var arr = new Array(n);
    for (var i = 0 ; i <n; i++) {
        arr[i] = "something" +i;
    }
    return arr;
}

var s = newShare("something", generateArray);

var er = "";
try {
	var p = newShare("wat", function() {return "whatever"});
	throw "the previous line should've errored";
} catch (e) {
	if (!e.toString().includes("only arrays can be made into shared objects")) {
		er = "wrong error " + e.toString()
	}
}

if (er != "") {
	throw er
}

exports.default = function() {
	if (s[2] !== "something2") {
		throw new Error("bad s[2]="+s[2])
	}
	if (s.length != 50) {
		throw new Error("bad length " +_s.length)
	}

	var i = 0;
	for (var v of s) {
		if (v !== "something"+i) {
			throw new Error("bad v="+v+" for i="+i)
		}
		i++;
	}

	if (s.something != undefined) {
		throw "s.something should've been undefined but was " + s.something;
	}

	try {
		s.something = 21
		throw "the previous line should've errored s.something = 21"
	} catch(e) {
		if (!e.toString().includes("Host object field something cannot be made configurable")) {
			er = "wrong error " + e.toString()
		}
	}

	if (er != "") {
		throw er
	}

	try {
		s[1]= "21"

		throw "the previous line should've errored"
	} catch(e) {
		if (!e.toString().includes("Host object field 1 cannot be made configurable")) {
			er = "wrong error " + e.toString()
		}
	}

	if (er != "") {
		throw er
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

	r2, err := NewFromArchive(logger, r1.MakeArchive(), lib.RuntimeOptions{})
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
			samples := make(chan stats.SampleContainer, 100)
			initVU, err := r.NewVU(1, samples)
			if assert.NoError(t, err) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
				err := vu.RunOnce()
				assert.NoError(t, err)
				entries := hook.Drain()
				require.Len(t, entries, 0)
			}
		})
	}
}
