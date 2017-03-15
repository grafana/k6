package js

import (
	"context"
	"fmt"
	"testing"

	log "github.com/Sirupsen/logrus"
	logtest "github.com/Sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestConsoleLog(t *testing.T) {
	levels := map[string]log.Level{
		"log":   log.InfoLevel,
		"debug": log.DebugLevel,
		"info":  log.InfoLevel,
		"warn":  log.WarnLevel,
		"error": log.ErrorLevel,
	}
	argsets := map[string]log.Fields{
		`"a"`:       {"arg0": "a"},
		`"a","b"`:   {"arg0": "a", "arg1": "b"},
		`{a:1}`:     {"a": "1"},
		`{a:1,b:2}`: {"a": "1", "b": "2"},
		`"a",{a:1}`: {"arg0": "a", "a": "1"},
		`{a:1},"a"`: {"a": "1", "arg1": "a"},
	}
	for name, level := range levels {
		t.Run(name, func(t *testing.T) {
			rt, err := New()
			assert.NoError(t, err)

			logger, hook := logtest.NewNullLogger()
			logger.Level = log.DebugLevel
			_ = rt.VM.Set("__console__", &Console{logger})

			for args, fields := range argsets {
				t.Run(args, func(t *testing.T) {
					_ = rt.VM.Set("__initapi__", &InitAPI{r: rt})
					exp, err := rt.load("__snippet__", []byte(fmt.Sprintf(`
					console.%s("init",%s);
					export default function() {
						console.%s("default",%s);
					}
					`, name, args, name, args)))
					if !assert.NoError(t, err) {
						return
					}
					_ = rt.VM.Set("__initapi__", nil)

					initEntry := hook.LastEntry()
					if assert.NotNil(t, initEntry, "nothing logged from init") {
						assert.Equal(t, "init", initEntry.Message)
						assert.Equal(t, level, initEntry.Level)
						assert.EqualValues(t, fields, initEntry.Data)
					}

					r, err := NewRunner(rt, exp)
					if !assert.NoError(t, err) {
						return
					}

					vu, err := r.NewVU()
					assert.NoError(t, err)

					_, err = vu.RunOnce(context.Background())
					assert.NoError(t, err)

					entry := hook.LastEntry()
					if assert.NotNil(t, entry, "nothing logged from default") {
						assert.Equal(t, "default", entry.Message)
						assert.Equal(t, level, entry.Level)
						assert.EqualValues(t, fields, entry.Data)
					}
				})
			}
		})
	}
}
