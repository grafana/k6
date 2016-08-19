package postman

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GeertJohan/go.rice"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
	_ "github.com/robertkrimen/otto/underscore"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"time"
)

var (
	mRequests = stats.Stat{Name: "requests", Type: stats.HistogramType, Intent: stats.TimeIntent}
	mErrors   = stats.Stat{Name: "errors", Type: stats.CounterType}
)

const vuSetupScript = `
	var globals = {};
	var environment = {};
	
	var postman = {};
	
	postman.setEnvironmentVariable = function(name, value) {
		environment[name] = value;
	}
	postman.setGlobalVariable = function(name, value) {
		globals[name] = value;
	}
	postman.clearEnvironmentVariable = function(name) {
		delete environment[name];
	}
	postman.clearGlobalVariable = function(name) {
		delete globals[name];
	}
	postman.clearEnvironmentVariables = function() {
		environment = {};
	}
	postman.clearGlobalVariables = function() {
		globals = {};
	}
	
	postman.getResponseHeader = function(name) {
		// Normalize captialization; "content-type"/"CONTENT-TYPE" -> "Content-Type"
		return responseHeaders[name.toLowerCase().replace(/(?:^|-)(\w)/g, function(txt) {
			return txt.toUpperCase();
		})];
	}
`

var libFiles = []string{
	"sugar/release/sugar.js",
}

var libPatches = map[string]map[string]string{
	"sugar/release/sugar.js": map[string]string{
		// Patch out functions using unsupported regex features.
		`function cleanDateInput(str) {
      str = str.trim().replace(/^just (?=now)|\.+$/i, '');
      return convertAsianDigits(str);
    }`: "",
		`function truncateOnWord(str, limit, fromLeft) {
      if (fromLeft) {
        return reverseString(truncateOnWord(reverseString(str), limit));
      }
      var reg = RegExp('(?=[' + getTrimmableCharacters() + '])');
      var words = str.split(reg);
      var count = 0;
      return words.filter(function(word) {
        count += word.length;
        return count <= limit;
      }).join('');
    }`: "",
		// We don't need to fully patch out this one, we just have to drop support for -昨 (last...)
		// This regex is only used to tell whether a character with multiple meanings is used as a
		// number or as a word, which is not something we're expecting people to do here anyways.
		`AsianDigitReg = RegExp('([期週周])?([' + KanjiDigits + FullWidthDigits + ']+)(?!昨)', 'g');`: `AsianDigitReg = RegExp('([期週周])?([' + KanjiDigits + FullWidthDigits + ']+)', 'g');`,
	},
}

type ErrorWithLineNumber struct {
	Wrapped error
	Line    int
}

func (e ErrorWithLineNumber) Error() string {
	return fmt.Sprintf("%s (line %d)", e.Wrapped.Error(), e.Line)
}

type Runner struct {
	VM         *otto.Otto
	Collection Collection
	Endpoints  []Endpoint
}

type VU struct {
	Runner    *Runner
	VM        *otto.Otto
	Client    http.Client
	Collector *stats.Collector
	Iteration int64
}

func New(source []byte) (*Runner, error) {
	var collection Collection
	if err := json.Unmarshal(source, &collection); err != nil {
		switch e := err.(type) {
		case *json.SyntaxError:
			src := string(source)
			line := strings.Count(src[:e.Offset], "\n") + 1
			return nil, ErrorWithLineNumber{Wrapped: e, Line: line}
		case *json.UnmarshalTypeError:
			src := string(source)
			line := strings.Count(src[:e.Offset], "\n") + 1
			return nil, ErrorWithLineNumber{Wrapped: e, Line: line}
		}
		return nil, err
	}

	vm := otto.New()
	lib, err := rice.FindBox("lib")
	if err != nil {
		return nil, errors.New(fmt.Sprintf("couldn't find postman lib files; this can happen if you run from the wrong working directory with a non-boxed binary: %s", err.Error()))
	}
	for _, filename := range libFiles {
		src, err := lib.String(filename)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("couldn't load lib file (%s): %s", filename, err.Error()))
		}
		for find, repl := range libPatches[filename] {
			src = strings.Replace(src, find, repl, 1)
		}
		if _, err := vm.Eval(src); err != nil {
			return nil, errors.New(fmt.Sprintf("couldn't eval lib file (%s): %s", filename, err.Error()))
		}
	}

	eps, err := MakeEndpoints(collection, vm)
	if err != nil {
		return nil, err
	}

	return &Runner{
		Collection: collection,
		Endpoints:  eps,
		VM:         vm,
	}, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	return &VU{
		Runner: r,
		VM:     r.VM.Copy(),
		Client: http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: math.MaxInt32,
			},
		},
		Collector: stats.NewCollector(),
	}, nil
}

func (u *VU) Reconfigure(id int64) error {
	u.Iteration = 0

	return nil
}

func (u *VU) RunOnce(ctx context.Context) error {
	u.Iteration++
	u.VM.Set("iteration", u.Iteration)

	if _, err := u.VM.Run(vuSetupScript); err != nil {
		return err
	}

	for _, ep := range u.Runner.Endpoints {
		req := ep.Request()

		startTime := time.Now()
		res, err := u.Client.Do(&req)
		duration := time.Since(startTime)

		var status int
		var body []byte
		if err == nil {
			status = res.StatusCode
			body, err = ioutil.ReadAll(res.Body)
			if err != nil {
				res.Body.Close()
				return err
			}
			res.Body.Close()
		}

		tags := stats.Tags{"method": ep.Method, "url": ep.URLString, "status": status}
		u.Collector.Add(stats.Sample{
			Stat:   &mRequests,
			Tags:   tags,
			Values: stats.Values{"duration": float64(duration)},
		})

		if err != nil {
			log.WithError(err).Error("Request error")
			u.Collector.Add(stats.Sample{
				Stat:   &mErrors,
				Tags:   tags,
				Values: stats.Value(1),
			})
			return err
		}

		if len(ep.Tests) > 0 {
			u.VM.Set("request", map[string]interface{}{
				"data":    ep.BodyMap,
				"headers": ep.HeaderMap,
				"method":  ep.Method,
				"url":     ep.URLString,
			})

			responseHeaders := make(map[string]string)
			for key, values := range res.Header {
				responseHeaders[key] = strings.Join(values, ", ")
			}
			u.VM.Set("responseHeaders", responseHeaders)

			// JSON seems to be geting automatically decoded by Postman? Is it decided by
			// Content-Type? Always attempted? We don't know, because it's nowhere in the docs!
			var obj interface{}
			if err := json.Unmarshal(body, &obj); err != nil {
				u.VM.Set("responseBody", string(body))
			} else {
				log.WithField("body", obj).Info("Body")
				u.VM.Set("responseBody", obj)
			}

			u.VM.Set("responseTime", duration/time.Millisecond)
			u.VM.Set("responseCode", map[string]interface{}{
				"code":   res.StatusCode,
				"name":   res.Status,
				"detail": res.Status, // The docs are vague on this one
			})
			u.VM.Set("tests", map[string]interface{}{})

			for _, script := range ep.Tests {
				if _, err := u.VM.Run(script); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
