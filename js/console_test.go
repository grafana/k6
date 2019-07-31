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
	"io/ioutil"
	"net/url"
	"os"
	"testing"

	null "gopkg.in/guregu/null.v3"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/loader"
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
	rt.Set("console", common.Bind(rt, &console{logger}, ctxPtr))

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
func getSimpleRunner(path, data string) (*Runner, error) {
	return getSimpleRunnerWithFileFs(path, data, afero.NewMemMapFs())
}

func getSimpleRunnerWithOptions(path, data string, options lib.RuntimeOptions) (*Runner, error) {
	return New(&loader.SourceData{
		URL:  &url.URL{Path: path, Scheme: "file"},
		Data: []byte(data),
	}, map[string]afero.Fs{
		"file":  afero.NewMemMapFs(),
		"https": afero.NewMemMapFs()},
		options)
}

func getSimpleRunnerWithFileFs(path, data string, fileFs afero.Fs) (*Runner, error) {
	return New(&loader.SourceData{
		URL:  &url.URL{Path: path, Scheme: "file"},
		Data: []byte(data),
	}, map[string]afero.Fs{
		"file":  fileFs,
		"https": afero.NewMemMapFs()},
		lib.RuntimeOptions{})
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
		name, level := name, level
		t.Run(name, func(t *testing.T) {
			for args, result := range argsets {
				args, result := args, result
				t.Run(args, func(t *testing.T) {
					r, err := getSimpleRunner("/script.js", fmt.Sprintf(
						`export default function() { console.%s(%s); }`,
						name, args,
					))
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

func TestFileConsole(t *testing.T) {
	var (
		levels = map[string]log.Level{
			"log":   log.InfoLevel,
			"debug": log.DebugLevel,
			"info":  log.InfoLevel,
			"warn":  log.WarnLevel,
			"error": log.ErrorLevel,
		}
		argsets = map[string]struct {
			Message string
			Data    log.Fields
		}{
			`"string"`:         {Message: "string"},
			`"string","a","b"`: {Message: "string", Data: log.Fields{"0": "a", "1": "b"}},
			`"string",1,2`:     {Message: "string", Data: log.Fields{"0": "1", "1": "2"}},
			`{}`:               {Message: "[object Object]"},
		}
		preExisting = map[string]bool{
			"log exists":        false,
			"log doesn't exist": true,
		}
		preExistingText = "Prexisting file\n"
	)
	for name, level := range levels {
		t.Run(name, func(t *testing.T) {
			for args, result := range argsets {
				t.Run(args, func(t *testing.T) {
					// whether the file is existed before logging
					for msg, deleteFile := range preExisting {
						t.Run(msg, func(t *testing.T) {
							var f, err = ioutil.TempFile("", "")
							if err != nil {
								t.Fatalf("Couldn't create temporary file for testing: %s", err)
							}
							var logFilename = f.Name()
							defer os.Remove(logFilename)
							// close it as we will want to reopen it and maybe remove it
							if deleteFile {
								f.Close()
								if err := os.Remove(logFilename); err != nil {
									t.Fatalf("Couldn't remove tempfile: %s", err)
								}
							} else {
								// TODO: handle case where the string was no written in full ?
								_, err := f.WriteString(preExistingText)
								f.Close()
								if err != nil {
									t.Fatalf("Error while writing text to preexisting logfile: %s", err)
								}

							}
							r, err := getSimpleRunner("/script",
								fmt.Sprintf(
									`export default function() { console.%s(%s); }`,
									name, args,
								))
							assert.NoError(t, err)

							err = r.SetOptions(lib.Options{
								ConsoleOutput: null.StringFrom(logFilename),
							})
							assert.NoError(t, err)

							samples := make(chan stats.SampleContainer, 100)
							vu, err := r.newVU(samples)
							assert.NoError(t, err)

							vu.Console.Logger.Level = log.DebugLevel
							hook := logtest.NewLocal(vu.Console.Logger)

							err = vu.RunOnce(context.Background())
							assert.NoError(t, err)

							// Test if the file was created.
							_, err = os.Stat(logFilename)
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

								// Test if what we logged to the hook is the same as what we logged
								// to the file.
								entryStr, err := entry.String()
								assert.NoError(t, err)

								f, err := os.Open(logFilename)
								assert.NoError(t, err)

								fileContent, err := ioutil.ReadAll(f)
								assert.NoError(t, err)

								var expectedStr = entryStr
								if !deleteFile {
									expectedStr = preExistingText + expectedStr
								}
								assert.Equal(t, expectedStr, string(fileContent))
							}

						})
					}
				})
			}
		})
	}
}
