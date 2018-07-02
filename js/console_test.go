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
	"context"
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestConsoleContext(t *testing.T) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	ctxPtr := new(context.Context)
	logger, hook := logtest.NewNullLogger()
	rt.Set("console", common.Bind(rt, &Console{logger}, ctxPtr))

	_, err := common.RunString(rt, `console.log("a")`)
	assert.NoError(t, err)
	if entry := hook.LastEntry(); assert.NotNil(t, entry) {
		assert.Equal(t, "a", entry.Message)
	}

	ctx, cancel := context.WithCancel(context.Background())
	*ctxPtr = ctx
	_, err = common.RunString(rt, `console.log("b")`)
	assert.NoError(t, err)
	if entry := hook.LastEntry(); assert.NotNil(t, entry) {
		assert.Equal(t, "b", entry.Message)
	}

	cancel()
	_, err = common.RunString(rt, `console.log("c")`)
	assert.NoError(t, err)
	if entry := hook.LastEntry(); assert.NotNil(t, entry) {
		assert.Equal(t, "b", entry.Message)
	}
}

func TestConsole(t *testing.T) {
	levels := map[string]log.Level{
		"log":   log.InfoLevel,
		"debug": log.DebugLevel,
		"info":  log.InfoLevel,
		"warn":  log.WarnLevel,
		"error": log.ErrorLevel,
	}
	argsets := map[string]struct {
		Message string
		Data    log.Fields
	}{
		`"string"`:         {Message: "string"},
		`"string","a","b"`: {Message: "string", Data: log.Fields{"0": "a", "1": "b"}},
		`"string",1,2`:     {Message: "string", Data: log.Fields{"0": "1", "1": "2"}},
		`{}`:               {Message: "[object Object]"},
	}
	for name, level := range levels {
		t.Run(name, func(t *testing.T) {
			for args, result := range argsets {
				t.Run(args, func(t *testing.T) {
					r, err := New(&lib.SourceData{
						Filename: "/script",
						Data: []byte(fmt.Sprintf(
							`export default function() { console.%s(%s); }`,
							name, args,
						)),
					}, afero.NewMemMapFs(), lib.RuntimeOptions{})
					assert.NoError(t, err)

					samples := make(chan stats.SampleContainer, 100)
					vu, err := r.newVU(samples)
					assert.NoError(t, err)

					logger, hook := logtest.NewNullLogger()
					logger.Level = log.DebugLevel
					vu.Console.Logger = logger

					err = vu.RunOnce(context.Background())
					assert.NoError(t, err)

					entry := hook.LastEntry()
					if assert.NotNil(t, entry, "nothing logged") {
						assert.Equal(t, level, entry.Level)
						assert.Equal(t, result.Message, entry.Message)

						data := result.Data
						if data == nil {
							data = make(log.Fields)
						}
						assert.Equal(t, data, entry.Data)
					}
				})
			}
		})
	}
}
