// Heavily influenced by the fantastic work by @dop251 for https://github.com/dop251/goja

package tc39

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
	"github.com/loadimpact/k6/js/compiler"
	jslib "github.com/loadimpact/k6/js/lib"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

const (
	tc39BASE = "TestTC39/test262"
)

//nolint:gochecknoglobals
var (
	errInvalidFormat = errors.New("invalid file format")

	// ignorableTestError = newSymbol(stringEmpty)

	sabStub = goja.MustCompile("sabStub.js", `
		Object.defineProperty(this, "SharedArrayBuffer", {
			get: function() {
				throw IgnorableTestError;
			}
		});`,
		false)

	esIDPrefixAllowList = []string{
		"sec-array",
		"sec-%typedarray%",
		"sec-string",
		"sec-date",
		"sec-number",
		"sec-math",
		"sec-arraybuffer-length",
		"sec-arraybuffer",
		"sec-regexp",
		"sec-variable-statement",
		"sec-ecmascript-standard-built-in-objects",
	}

	featuresBlockList = []string{
		"BigInt",    // not supported at all
		"IsHTMLDDA", // not supported at all
	}
	skipList       = map[string]bool{}
	pathBasedBlock = map[string]bool{ // This completely skips any path matching it without any kind of message
		"test/annexB/built-ins/Date":                          true,
		"test/annexB/built-ins/RegExp/prototype/Symbol.split": true,
		"test/annexB/built-ins/String/prototype/anchor":       true,
		"test/annexB/built-ins/String/prototype/big":          true,
		"test/annexB/built-ins/String/prototype/blink":        true,
		"test/annexB/built-ins/String/prototype/bold":         true,
		"test/annexB/built-ins/String/prototype/fixed":        true,
		"test/annexB/built-ins/String/prototype/fontcolor":    true,
		"test/annexB/built-ins/String/prototype/fontsize":     true,
		"test/annexB/built-ins/String/prototype/italics":      true,
		"test/annexB/built-ins/String/prototype/link":         true,
		"test/annexB/built-ins/String/prototype/small":        true,
		"test/annexB/built-ins/String/prototype/strike":       true,
		"test/annexB/built-ins/String/prototype/sub":          true,
		"test/annexB/built-ins/String/prototype/sup":          true,

		// Async/Promise and other totally unsupported functionality
		"test/built-ins/AsyncArrowFunction":             true,
		"test/built-ins/AsyncFromSyncIteratorPrototype": true,
		"test/built-ins/AsyncFunction":                  true,
		"test/built-ins/AsyncGeneratorFunction":         true,
		"test/built-ins/AsyncGeneratorPrototype":        true,
		"test/built-ins/AsyncIteratorPrototype":         true,
		"test/built-ins/Atomics":                        true,
		"test/built-ins/BigInt":                         true,
		"test/built-ins/Promise":                        true,
		"test/built-ins/SharedArrayBuffer":              true,

		"test/built-ins/Date/parse/without-utc-offset.js": true, // some other reason ?!? depending on local time

		"test/built-ins/Array/prototype/concat/arg-length-exceeding-integer-limit.js": true, // takes forever and is broken
		"test/built-ins/Array/prototype/splice/throws-if-integer-limit-exceeded.js":   true, // takes forever and is broken
		"test/built-ins/Array/prototype/unshift/clamps-to-integer-limit.js":           true, // takes forever and is broken
		"test/built-ins/Array/prototype/unshift/throws-if-integer-limit-exceeded.js":  true, // takes forever and is broken
	}
)

//nolint:unused,structcheck
type tc39Test struct {
	name string
	f    func(t *testing.T)
}

type tc39BenchmarkItem struct {
	name     string
	duration time.Duration
}

type tc39BenchmarkData []tc39BenchmarkItem

type tc39TestCtx struct {
	compiler       *compiler.Compiler
	base           string
	t              *testing.T
	prgCache       map[string]*goja.Program
	prgCacheLock   sync.Mutex
	enableBench    bool
	benchmark      tc39BenchmarkData
	benchLock      sync.Mutex
	testQueue      []tc39Test //nolint:unused,structcheck
	expectedErrors map[string]string

	errorsLock sync.Mutex
	errors     map[string]string
}

type TC39MetaNegative struct {
	Phase, Type string
}

type tc39Meta struct {
	Negative TC39MetaNegative
	Includes []string
	Flags    []string
	Features []string
	Es5id    string
	Es6id    string
	Esid     string
}

func (m *tc39Meta) hasFlag(flag string) bool {
	for _, f := range m.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

func parseTC39File(name string) (*tc39Meta, string, error) {
	f, err := os.Open(name) //nolint:gosec
	if err != nil {
		return nil, "", err
	}
	defer f.Close() //nolint:errcheck,gosec

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, "", err
	}

	metaStart := bytes.Index(b, []byte("/*---"))
	if metaStart == -1 {
		return nil, "", errInvalidFormat
	}

	metaStart += 5
	metaEnd := bytes.Index(b, []byte("---*/"))
	if metaEnd == -1 || metaEnd <= metaStart {
		return nil, "", errInvalidFormat
	}

	var meta tc39Meta
	err = yaml.Unmarshal(b[metaStart:metaEnd], &meta)
	if err != nil {
		return nil, "", err
	}

	if meta.Negative.Type != "" && meta.Negative.Phase == "" {
		return nil, "", errors.New("negative type is set, but phase isn't")
	}

	return &meta, string(b), nil
}

func (*tc39TestCtx) detachArrayBuffer(call goja.FunctionCall) goja.Value {
	if obj, ok := call.Argument(0).(*goja.Object); ok {
		var buf goja.ArrayBuffer
		if goja.New().ExportTo(obj, &buf) == nil {
			// if buf, ok := obj.Export().(goja.ArrayBuffer); ok {
			buf.Detach()
			return goja.Undefined()
		}
	}
	panic(goja.New().NewTypeError("detachArrayBuffer() is called with incompatible argument"))
}

func (ctx *tc39TestCtx) fail(t testing.TB, name string, strict bool, errStr string) {
	nameKey := fmt.Sprintf("%s-strict:%v", name, strict)
	expected, ok := ctx.expectedErrors[nameKey]
	if ok {
		if !assert.Equal(t, expected, errStr) {
			ctx.errorsLock.Lock()
			fmt.Println("different")
			fmt.Println(expected)
			fmt.Println(errStr)
			ctx.errors[nameKey] = errStr
			ctx.errorsLock.Unlock()
		}
	} else {
		assert.Empty(t, errStr)
		ctx.errorsLock.Lock()
		fmt.Println("no error", name)
		ctx.errors[nameKey] = errStr
		ctx.errorsLock.Unlock()
	}
}

func (ctx *tc39TestCtx) runTC39Test(t testing.TB, name, src string, meta *tc39Meta, strict bool) {
	if skipList[name] {
		t.Skip("Excluded")
	}
	failf := func(str string, args ...interface{}) {
		str = fmt.Sprintf(str, args)
		ctx.fail(t, name, strict, str)
	}
	defer func() {
		if x := recover(); x != nil {
			failf("panic while running %s: %v", name, x)
		}
	}()
	vm := goja.New()
	_262 := vm.NewObject()
	ignorableTestError := vm.NewGoError(fmt.Errorf(""))
	vm.Set("IgnorableTestError", ignorableTestError)
	_ = _262.Set("detachArrayBuffer", ctx.detachArrayBuffer)
	_ = _262.Set("createRealm", func(goja.FunctionCall) goja.Value {
		panic(ignorableTestError)
	})
	vm.Set("$262", _262)
	vm.Set("print", t.Log)
	if _, err := vm.RunProgram(jslib.GetCoreJS()); err != nil {
		panic(err)
	}
	_, err := vm.RunProgram(sabStub)
	if err != nil {
		panic(err)
	}
	if strict {
		src = "'use strict';\n" + src
	}
	early, err := ctx.runTC39Script(name, src, meta.Includes, vm)

	if err == nil {
		if meta.Negative.Type != "" {
			// vm.vm.prg.dumpCode(t.Logf)
			failf("%s: Expected error: %v", name, err)
			return
		}
		nameKey := fmt.Sprintf("%s-strict:%v", name, strict)
		expected, ok := ctx.expectedErrors[nameKey]
		assert.False(t, ok, "%s passes but and error %q was expected", nameKey, expected)
		return
	}

	if meta.Negative.Type == "" {
		if err, ok := err.(*goja.Exception); ok {
			if err.Value() == ignorableTestError {
				t.Skip("Test threw IgnorableTestError")
			}
		}
		failf("%s: %v", name, err)
		return
	}
	if meta.Negative.Phase == "early" && !early || meta.Negative.Phase == "runtime" && early {
		failf("%s: error %v happened at the wrong phase (expected %s)", name, err, meta.Negative.Phase)
		return
	}
	var errType string

	switch err := err.(type) {
	case *goja.Exception:
		if o, ok := err.Value().(*goja.Object); ok { //nolint:nestif
			if c := o.Get("constructor"); c != nil {
				if c, ok := c.(*goja.Object); ok {
					errType = c.Get("name").String()
				} else {
					failf("%s: error constructor is not an object (%v)", name, o)
					return
				}
			} else {
				failf("%s: error does not have a constructor (%v)", name, o)
				return
			}
		} else {
			failf("%s: error is not an object (%v)", name, err.Value())
			return
		}
	case *goja.CompilerSyntaxError, *parser.Error, parser.ErrorList:
		errType = "SyntaxError"
	case *goja.CompilerReferenceError:
		errType = "ReferenceError"
	default:
		failf("%s: error is not a JS error: %v", name, err)
		return
	}

	_ = errType
	if errType != meta.Negative.Type {
		// vm.vm.prg.dumpCode(t.Logf)
		failf("%s: unexpected error type (%s), expected (%s)", name, errType, meta.Negative.Type)
		return
	}

	/*
		if vm.vm.sp != 0 {
			t.Fatalf("sp: %d", vm.vm.sp)
		}

		if l := len(vm.vm.iterStack); l > 0 {
			t.Fatalf("iter stack is not empty: %d", l)
		}
	*/
}

func shouldBeSkipped(t testing.TB, meta *tc39Meta) {
	if meta.hasFlag("async") { // this is totally not supported
		t.Skipf("Skipping as it has flag async")
	}
	if meta.Es6id == "" && meta.Es5id == "" { //nolint:nestif
		skip := true

		if skip {
			if meta.Esid != "" {
				for _, prefix := range esIDPrefixAllowList {
					if strings.HasPrefix(meta.Esid, prefix) {
						skip = false
					}
				}
			}
		}
		for _, feature := range meta.Features {
			for _, bl := range featuresBlockList {
				if feature == bl {
					t.Skipf("Blocklisted feature %s", feature)
				}
			}
		}
		if skip {
			t.Skipf("Not ES6 or ES5 esid: %s", meta.Esid)
		}
	}
}

func (ctx *tc39TestCtx) runTC39File(name string, t testing.TB) {
	p := path.Join(ctx.base, name)
	meta, src, err := parseTC39File(p)
	if err != nil {
		// t.Fatalf("Could not parse %s: %v", name, err)
		t.Errorf("Could not parse %s: %v", name, err)
		return
	}

	shouldBeSkipped(t, meta)

	var startTime time.Time
	if ctx.enableBench {
		startTime = time.Now()
	}

	hasRaw := meta.hasFlag("raw")

	/*
		if hasRaw || !meta.hasFlag("onlyStrict") {
			// log.Printf("Running normal test: %s", name)
			// t.Logf("Running normal test: %s", name)
			ctx.runTC39Test(t, name, src, meta, false)
		}
	*/

	if !hasRaw && !meta.hasFlag("noStrict") {
		// log.Printf("Running strict test: %s", name)
		// t.Logf("Running strict test: %s", name)
		ctx.runTC39Test(t, name, src, meta, true)
	} else { // Run test in non strict mode only if we won't run them in strict
		// TODO uncomment the if above and delete this else so we run both parts when the tests
		// don't take forever
		ctx.runTC39Test(t, name, src, meta, false)
	}

	if ctx.enableBench {
		ctx.benchLock.Lock()
		ctx.benchmark = append(ctx.benchmark, tc39BenchmarkItem{
			name:     name,
			duration: time.Since(startTime),
		})
		ctx.benchLock.Unlock()
	}
}

func (ctx *tc39TestCtx) init() {
	ctx.prgCache = make(map[string]*goja.Program)
	ctx.errors = make(map[string]string)

	b, err := ioutil.ReadFile("./breaking_test_errors.json")
	if err != nil {
		panic(err)
	}
	ctx.expectedErrors = make(map[string]string, 1000)
	err = json.Unmarshal(b, &ctx.expectedErrors)
	if err != nil {
		panic(err)
	}
}

func (ctx *tc39TestCtx) compile(base, name string) (*goja.Program, error) {
	ctx.prgCacheLock.Lock()
	defer ctx.prgCacheLock.Unlock()

	prg := ctx.prgCache[name]
	if prg == nil {
		fname := path.Join(base, name)
		f, err := os.Open(fname) //nolint:gosec
		if err != nil {
			return nil, err
		}
		defer f.Close() //nolint:gosec,errcheck

		b, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}

		str := string(b)
		prg, _, err = ctx.compiler.Compile(str, name, "", "", false, lib.CompatibilityModeExtended)
		if err != nil {
			return nil, err
		}
		ctx.prgCache[name] = prg
	}

	return prg, nil
}

func (ctx *tc39TestCtx) runFile(base, name string, vm *goja.Runtime) error {
	prg, err := ctx.compile(base, name)
	if err != nil {
		return err
	}
	_, err = vm.RunProgram(prg)
	return err
}

func (ctx *tc39TestCtx) runTC39Script(name, src string, includes []string, vm *goja.Runtime) (early bool, err error) {
	early = true
	err = ctx.runFile(ctx.base, path.Join("harness", "assert.js"), vm)
	if err != nil {
		return
	}

	err = ctx.runFile(ctx.base, path.Join("harness", "sta.js"), vm)
	if err != nil {
		return
	}

	for _, include := range includes {
		err = ctx.runFile(ctx.base, path.Join("harness", include), vm)
		if err != nil {
			return
		}
	}

	var p *goja.Program
	p, _, err = ctx.compiler.Compile(src, name, "", "", false, lib.CompatibilityModeExtended)

	if err != nil {
		return
	}

	early = false
	_, err = vm.RunProgram(p)

	return
}

func (ctx *tc39TestCtx) runTC39Tests(name string) {
	files, err := ioutil.ReadDir(path.Join(ctx.base, name))
	if err != nil {
		ctx.t.Fatal(err)
	}

	for _, file := range files {
		if file.Name()[0] == '.' {
			continue
		}
		newName := path.Join(name, file.Name())
		if pathBasedBlock[newName] {
			ctx.t.Run(newName, func(t *testing.T) {
				t.Skipf("Skip %s beause of path based block", newName)
			})
			continue
		}
		if file.IsDir() {
			ctx.runTC39Tests(newName)
		} else if strings.HasSuffix(file.Name(), ".js") && !strings.HasSuffix(file.Name(), "_FIXTURE.js") {
			ctx.runTest(newName, func(t *testing.T) {
				ctx.runTC39File(newName, t)
			})
		}
	}
}

func TestTC39(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	if _, err := os.Stat(tc39BASE); err != nil {
		t.Skipf("If you want to run tc39 tests, you need to run the 'checkout.sh` script in the directory to get  https://github.com/tc39/test262 at the correct last tested commit (%v)", err)
	}

	ctx := &tc39TestCtx{
		base:     tc39BASE,
		compiler: compiler.New(testutils.NewLogger(t)),
	}
	ctx.init()
	// ctx.enableBench = true

	t.Run("test262", func(t *testing.T) {
		ctx.t = t
		ctx.runTC39Tests("test/language")
		ctx.runTC39Tests("test/built-ins")
		ctx.runTC39Tests("test/harness")
		ctx.runTC39Tests("test/annexB/built-ins")

		ctx.flush()
	})

	if ctx.enableBench {
		sort.Slice(ctx.benchmark, func(i, j int) bool {
			return ctx.benchmark[i].duration > ctx.benchmark[j].duration
		})
		bench := ctx.benchmark
		if len(bench) > 50 {
			bench = bench[:50]
		}
		for _, item := range bench {
			fmt.Printf("%s\t%d\n", item.name, item.duration/time.Millisecond)
		}
	}
	if len(ctx.errors) > 0 {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(ctx.errors)
	}
}
