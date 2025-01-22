// Heavily influenced by the fantastic work by @dop251 for https://github.com/dop251/goja
//
//nolint:forbidigo
package tc39

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/grafana/sobek/parser"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"go.k6.io/k6/internal/js/compiler"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
)

const (
	tc39BASE = "TestTC39/test262"
)

//nolint:gochecknoglobals
var (
	errInvalidFormat = errors.New("invalid file format")

	// ignorableTestError = newSymbol(stringEmpty)

	sabStub = sobek.MustCompile("sabStub.js", `
		Object.defineProperty(this, "SharedArrayBuffer", {
			get: function() {
				throw IgnorableTestError;
			}
		});`,
		false)

	featuresBlockList = []string{
		"IsHTMLDDA",                   // not supported at all
		"async-iteration",             // not supported at all
		"String.prototype.replaceAll", // not supported at all, Stage 4 since 2020
		"dynamic-import",              // not support in k6

		// from Sobek
		"Symbol.asyncIterator",
		"resizable-arraybuffer",
		"regexp-named-groups",
		"regexp-duplicate-named-groups",
		"regexp-unicode-property-escapes",
		"regexp-match-indices",
		"regexp-modifiers",
		"RegExp.escape",
		"legacy-regexp",
		"tail-call-optimization",
		"Temporal",
		"import-assertions",
		"logical-assignment-operators",
		"Atomics",
		"Atomics.waitAsync",
		"Atomics.pause",
		"FinalizationRegistry",
		"WeakRef",
		"__getter__",
		"__setter__",
		"ShadowRealm",
		"SharedArrayBuffer",
		"decorators",

		"regexp-duplicate-named-groups",
		"regexp-v-flag",
		"iterator-helpers",
		"symbols-as-weakmap-keys",
		"uint8array-base64",
		"String.prototype.toWellFormed",
		"explicit-resource-management",
		"set-methods",
		"promise-try",
		"promise-with-resolvers",
		"array-grouping",
		"Math.sumPrecise",
		"Float16Array",
		"arraybuffer-transfer",
		"Array.fromAsync",
		"String.prototype.isWellFormed",

		"source-phase-imports",
		"import-attributes",
	}
	skipWords = []string{}
	skipList  = map[string]bool{
		"test/built-ins/Date/parse/without-utc-offset.js": true, // some other reason ?!? depending on local time

		"test/built-ins/Array/prototype/concat/arg-length-exceeding-integer-limit.js": true, // takes forever and is broken
		"test/built-ins/Array/prototype/splice/throws-if-integer-limit-exceeded.js":   true, // takes forever and is broken
		"test/built-ins/Array/prototype/unshift/clamps-to-integer-limit.js":           true, // takes forever and is broken
		"test/built-ins/Array/prototype/unshift/throws-if-integer-limit-exceeded.js":  true, // takes forever and is broken

		// async generator
		"test/built-ins/Object/seal/seal-asyncgeneratorfunction.js": true,
		"test/language/expressions/async-generator/name.js":         true,

		// wrong tests after change in the specification https://github.com/tc39/test262/issues/3407
		"test/language/expressions/prefix-decrement/S11.4.5_A6_T3.js":        true,
		"test/language/expressions/prefix-decrement/S11.4.5_A6_T2.js":        true,
		"test/language/expressions/prefix-decrement/S11.4.5_A6_T1.js":        true,
		"test/language/expressions/postfix-increment/S11.3.1_A6_T3.js":       true,
		"test/language/expressions/postfix-increment/S11.3.1_A6_T2.js":       true,
		"test/language/expressions/postfix-increment/S11.3.1_A6_T1.js":       true,
		"test/language/expressions/postfix-decrement/S11.3.2_A6_T3.js":       true,
		"test/language/expressions/postfix-decrement/S11.3.2_A6_T1.js":       true,
		"test/language/expressions/postfix-decrement/S11.3.2_A6_T2.js":       true,
		"test/language/expressions/prefix-increment/S11.4.4_A6_T3.js":        true,
		"test/language/expressions/prefix-increment/S11.4.4_A6_T2.js":        true,
		"test/language/expressions/prefix-increment/S11.4.4_A6_T1.js":        true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.9_T2.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.9_T4.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.9_T1.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.8_T4.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.8_T2.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.8_T1.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.7_T4.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.7_T2.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.7_T1.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.6_T4.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.6_T2.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.6_T1.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.5_T4.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.5_T2.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.5_T1.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.4_T4.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.3_T4.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.4_T1.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.4_T2.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.3_T2.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.3_T1.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.2_T4.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.2_T2.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.2_T1.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.1_T4.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.1_T2.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.11_T4.js": true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.11_T2.js": true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.1_T1.js":  true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.11_T1.js": true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.10_T4.js": true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.10_T2.js": true,
		"test/language/expressions/compound-assignment/S11.13.2_A7.10_T1.js": true,
		"test/language/expressions/assignment/S11.13.1_A7_T3.js":             true,

		// timezone (apparently it depends on local timezone settings)
		"test/built-ins/Date/prototype/toISOString/15.9.5.43-0-8.js":  true,
		"test/built-ins/Date/prototype/toISOString/15.9.5.43-0-9.js":  true,
		"test/built-ins/Date/prototype/toISOString/15.9.5.43-0-10.js": true,
	}
	pathBasedBlock = []string{ // This completely skips any path matching it without any kind of message
		"test/annexB/built-ins/Date",
		"test/annexB/built-ins/RegExp/prototype/Symbol.split",
		"test/annexB/built-ins/String/prototype/anchor",
		"test/annexB/built-ins/String/prototype/big",
		"test/annexB/built-ins/String/prototype/blink",
		"test/annexB/built-ins/String/prototype/bold",
		"test/annexB/built-ins/String/prototype/fixed",
		"test/annexB/built-ins/String/prototype/fontcolor",
		"test/annexB/built-ins/String/prototype/fontsize",
		"test/annexB/built-ins/String/prototype/italics",
		"test/annexB/built-ins/String/prototype/link",
		"test/annexB/built-ins/String/prototype/small",
		"test/annexB/built-ins/String/prototype/strike",
		"test/annexB/built-ins/String/prototype/sub",
		"test/annexB/built-ins/String/prototype/sup",

		"test/annexB/built-ins/RegExp/legacy-accessors/",
		"test/language/literals/string/legacy-", // legecy string escapes

		"test/built-ins/Atomics",
		"test/built-ins/BigInt",
		"test/built-ins/SharedArrayBuffer",

		// Async generators are not currently supported
		"test/built-ins/AsyncFromSyncIteratorPrototype",
		"test/built-ins/AsyncGeneratorFunction",
		"test/built-ins/AsyncGeneratorPrototype",
		"test/built-ins/AsyncIteratorPrototype",
		"test/language/eval-code/direct/async-gen",
		"test/language/expressions/async-generator",
		"test/language/expressions/class/elements/after-same-line-gen-rs-static-async-generator",
		"test/language/expressions/class/elements/after-same-line-method-rs-static-async-generator",
		"test/language/expressions/class/elements/after-same-line-static-async-method-rs-static-async-generator",
		"test/language/expressions/class/elements/after-same-line-static-gen-rs-static-async-generator",
		"test/language/expressions/class/elements/after-same-line-static-method-rs-static-async-generator",
		"test/language/expressions/class/elements/multiple-definitions-rs-static-async-generator",
		"test/language/expressions/class/elements/multiple-stacked-definitions-rs-static-async-generator",
		"test/language/expressions/class/elements/new-no-sc-line-gen-rs-static-async-generator",
		"test/language/expressions/class/elements/new-no-sc-line-method-rs-static-async-generator",
		"test/language/expressions/class/elements/new-sc-line-gen-rs-static-async-generator",
		"test/language/expressions/class/elements/new-sc-line-method-rs-static-async-generator",
		"test/language/expressions/class/elements/private-async-generator",
		"test/language/expressions/class/elements/private-static-async-generator",
		"test/language/expressions/class/elements/regular-definitions-rs-static-async-generator",
		"test/language/expressions/class/elements/same-line-async-method-rs-static-async-generator",
		"test/language/expressions/class/elements/same-line-gen-rs-static-async-generator",
		"test/language/expressions/class/elements/same-line-method-rs-static-async-generator",
		"test/language/expressions/class/elements/wrapped-in-sc-rs-static-async-generator",
		"test/language/statements/class/elements/after-same-line-gen-rs-static-async-generator",
		"test/language/statements/class/elements/after-same-line-method-rs-static-async-generator",
		"test/language/statements/class/elements/after-same-line-static-async-method-rs-static-async-generator",
		"test/language/statements/class/elements/after-same-line-static-gen-rs-static-async-generator",
		"test/language/statements/class/elements/after-same-line-static-method-rs-static-async-generator",
		"test/language/statements/class/elements/multiple-definitions-rs-static-async-generator",
		"test/language/statements/class/elements/multiple-stacked-definitions-rs-static-async-generator",
		"test/language/statements/class/elements/new-no-sc-line-method-rs-static-async-generator",
		"test/language/statements/class/elements/new-sc-line-gen-rs-static-async-generator",
		"test/language/statements/class/elements/new-sc-line-method-rs-static-async-generator",
		"test/language/statements/class/elements/private-async-generator",
		"test/language/statements/class/elements/private-static-async-generator",
		"test/language/statements/class/elements/regular-definitions-rs-static-async-generator",
		"test/language/statements/class/elements/same-line-async-method-rs-static-async-generator",
		"test/language/statements/class/elements/same-line-gen-rs-static-async-generator",
		"test/language/statements/class/elements/same-line-method-rs-static-async-generator",
		"test/language/statements/class/elements/wrapped-in-sc-rs-static-async-generator",
		"test/language/statements/switch/scope-lex-async-generator.js",

		"test/built-ins/FinalizationRegistry", // still in proposal

		"test/built-ins/RegExp/property-escapes",  // none of those work
		"test/language/identifiers/start-unicode", // tests whether some unicode can be used in identifiers, half don't work, take forever

		"test/built-ins/Object/prototype/__lookup", // AnnexB lookupGetter lookupSetter
		"test/built-ins/Object/prototype/__define", // AnnexB defineGetter defineSetter

		"test/language/expressions/dynamic-import", // not supported

		// Go 1.16 only supports unicode 13; While go 1.21 has support for this - we disable them
		// so that they aren't flaky between versions.
		"test/language/identifiers/start-unicode-14.",
		"test/language/identifiers/part-unicode-14.",
	}

	update = flag.Bool("update", false, "update breaking_test_errors-*.json files") //nolint:gochecknoglobals
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
	base           string
	t              *testing.T
	prgCache       map[string]*sobek.Program
	prgCacheLock   sync.Mutex
	enableBench    bool
	benchmark      tc39BenchmarkData
	benchLock      sync.Mutex
	testQueue      []tc39Test //nolint:unused,structcheck
	expectedErrors map[string]string

	errorsLock sync.Mutex
	errors     map[string]string

	compatibilityMode lib.CompatibilityMode
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
	defer f.Close() //nolint:errcheck

	b, err := io.ReadAll(f)
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

func (*tc39TestCtx) detachArrayBuffer(call sobek.FunctionCall) sobek.Value {
	if obj, ok := call.Argument(0).(*sobek.Object); ok {
		var buf sobek.ArrayBuffer
		if sobek.New().ExportTo(obj, &buf) == nil {
			// if buf, ok := obj.Export().(sobek.ArrayBuffer); ok {
			buf.Detach()
			return sobek.Undefined()
		}
	}
	panic(sobek.New().NewTypeError("detachArrayBuffer() is called with incompatible argument"))
}

func (ctx *tc39TestCtx) fail(t testing.TB, name string, strict bool, errStr string) {
	nameKey := fmt.Sprintf("%s-strict:%v", name, strict)
	expected, ok := ctx.expectedErrors[nameKey]
	if index := strings.LastIndex(errStr, " at "); index != -1 {
		errStr = errStr[:index] + " <at omitted>"
	}
	if ok {
		if !assert.Equal(t, expected, errStr) {
			ctx.errorsLock.Lock()
			ctx.errors[nameKey] = errStr
			ctx.errorsLock.Unlock()
		}
	} else {
		assert.Empty(t, errStr)
		ctx.errorsLock.Lock()
		ctx.errors[nameKey] = errStr
		ctx.errorsLock.Unlock()
	}
}

func (ctx *tc39TestCtx) runTC39Test(t testing.TB, name, src string, meta *tc39Meta, strict bool) {
	if skipList[name] {
		t.Skip("Excluded")
	}
	failf := func(str string, args ...interface{}) {
		str = fmt.Sprintf(str, args...)
		ctx.fail(t, name, strict, str)
	}
	defer func() {
		if x := recover(); x != nil {
			failf("panic while running %s: %v", name, x)
		}
	}()
	vm := sobek.New()
	_262 := vm.NewObject()
	ignorableTestError := vm.NewGoError(fmt.Errorf(""))
	_ = vm.Set("IgnorableTestError", ignorableTestError)
	_ = _262.Set("detachArrayBuffer", ctx.detachArrayBuffer)
	_ = _262.Set("createRealm", func(sobek.FunctionCall) sobek.Value {
		panic(ignorableTestError)
	})
	_ = _262.Set("evalScript", func(call sobek.FunctionCall) sobek.Value {
		script := call.Argument(0).String()
		result, err := vm.RunString(script)
		if err != nil {
			panic(err)
		}
		return result
	})

	_ = vm.Set("$262", _262)
	_ = vm.Set("print", t.Log)
	_, err := vm.RunProgram(sabStub)
	if err != nil {
		panic(err)
	}
	if strict {
		src = "'use strict';\n" + src
	}

	var out []string
	async := meta.hasFlag("async") //nolint:ifshort // false positive
	if async {
		err = ctx.runFile(ctx.base, path.Join("harness", "doneprintHandle.js"), vm)
		if err != nil {
			t.Fatal(err)
		}
		_ = vm.Set("print", func(msg string) {
			out = append(out, msg)
		})
	} else {
		_ = vm.Set("print", t.Log)
	}
	var early bool
	if meta.hasFlag("module") {
		early, err = ctx.runTC39Module(name, src, meta.Includes, vm)
	} else {
		early, err = ctx.runTC39Script(name, src, meta.Includes, vm, meta.Negative.Type != "")
	}

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
		if err, ok := err.(*sobek.Exception); ok { //nolint:errorlint
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
	errType := getErrType(name, err, failf)

	if errType != "" && errType != meta.Negative.Type {
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
	if async {
		complete := false
		for _, line := range out {
			if strings.HasPrefix(line, "Test262:AsyncTestFailure:") {
				t.Fatal(line)
			} else if line == "Test262:AsyncTestComplete" {
				complete = true
			}
		}
		if !complete {
			for _, line := range out {
				t.Log(line)
			}
			t.Fatal("Test262:AsyncTestComplete was not printed")
		}
	}
}

func getErrType(name string, err error, failf func(str string, args ...interface{})) string {
	switch err := err.(type) { //nolint:errorlint
	case *sobek.Exception:
		if o, ok := err.Value().(*sobek.Object); ok {
			if c := o.Get("constructor"); c != nil {
				if c, ok := c.(*sobek.Object); ok {
					return c.Get("name").String()
				}
				failf("%s: error constructor is not an object (%v)", name, o)
				return ""
			}
			failf("%s: error does not have a constructor (%v)", name, o)
			return ""
		}
		failf("%s: error is not an object (%v)", name, err.Value())
		return ""
	case *sobek.CompilerSyntaxError, *parser.Error, parser.ErrorList:
		return "SyntaxError"
	case *sobek.CompilerReferenceError:
		return "ReferenceError"
	default:
		failf("%s: error is not a JS error: %v", name, err)
		return ""
	}
}

func shouldBeSkipped(t testing.TB, meta *tc39Meta) {
	for _, feature := range meta.Features {
		for _, bl := range featuresBlockList {
			if feature == bl {
				t.Skipf("Blocklisted feature %s", feature)
			}
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
		// NOTE: in practice k6 does not run non strict code at all so this tests make no sense
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

func breakingTestErrorsFilename(compatibilityMode lib.CompatibilityMode) string {
	return fmt.Sprintf("./breaking_test_errors-%s.json", compatibilityMode)
}

func (ctx *tc39TestCtx) init() {
	ctx.prgCache = make(map[string]*sobek.Program)
	ctx.errors = make(map[string]string)

	if !*update {
		b, err := os.ReadFile(breakingTestErrorsFilename(ctx.compatibilityMode))
		if err != nil {
			panic(err)
		}
		b = bytes.TrimSpace(b)
		if len(b) > 0 {
			ctx.expectedErrors = make(map[string]string, 1000)
			err = json.Unmarshal(b, &ctx.expectedErrors)
			if err != nil {
				panic(err)
			}
		}
	}
}

func (ctx *tc39TestCtx) compile(base, name string) (*sobek.Program, error) {
	ctx.prgCacheLock.Lock()
	defer ctx.prgCacheLock.Unlock()

	prg := ctx.prgCache[name]
	if prg == nil {
		fname := path.Join(base, name)
		f, err := os.Open(fname) //nolint:gosec
		if err != nil {
			return nil, err
		}
		defer f.Close() //nolint:errcheck

		b, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}

		prg, err = ctx.compileOnly(string(b), name, ctx.compatibilityMode)
		if err != nil {
			return nil, err
		}
		ctx.prgCache[name] = prg
	}

	return prg, nil
}

func (ctx *tc39TestCtx) runFile(base, name string, vm *sobek.Runtime) error {
	prg, err := ctx.compile(base, name)
	if err != nil {
		return err
	}
	_, err = vm.RunProgram(prg)
	return err
}

func (ctx *tc39TestCtx) compileOnly(src, name string, compatibilityMode lib.CompatibilityMode) (*sobek.Program, error) {
	comp := ctx.compiler()
	if compatibilityMode == lib.CompatibilityModeExperimentalEnhanced {
		code, _, err := compiler.StripTypes(src, name)
		if err != nil {
			return nil, err
		}
		src = code
	}
	astProgram, _, err := comp.Parse(src, name, false, false)
	if err != nil {
		return nil, err
	}
	return sobek.CompileAST(astProgram, false)
}

func (ctx *tc39TestCtx) compiler() *compiler.Compiler {
	return compiler.New(testutils.NewLogger(ctx.t))
}

func (ctx *tc39TestCtx) runTC39Script(name, src string, includes []string, vm *sobek.Runtime, expectsError bool) (early bool, err error) {
	early = true
	err = ctx.runFile(ctx.base, path.Join("harness", "assert.js"), vm)
	if err != nil {
		return early, err
	}

	err = ctx.runFile(ctx.base, path.Join("harness", "sta.js"), vm)
	if err != nil {
		return early, err
	}

	for _, include := range includes {
		err = ctx.runFile(ctx.base, path.Join("harness", include), vm)
		if err != nil {
			return early, err
		}
	}

	p, err := ctx.compileOnly(src, name, lib.CompatibilityModeBase)
	if err != nil && !expectsError {
		p, err = ctx.compileOnly(src, name, lib.CompatibilityModeExtended)
	}
	if err != nil {
		return early, err
	}

	early = false
	_, err = vm.RunProgram(p)

	return early, err
}

func (ctx *tc39TestCtx) runTC39Module(name, src string, includes []string, vm *sobek.Runtime) (early bool, err error) {
	moduleRuntime := modulestest.NewRuntime(ctx.t)
	moduleRuntime.VU.RuntimeField = vm
	early = true
	err = ctx.runFile(ctx.base, path.Join("harness", "assert.js"), vm)
	if err != nil {
		return early, err
	}

	err = ctx.runFile(ctx.base, path.Join("harness", "sta.js"), vm)
	if err != nil {
		return early, err
	}

	for _, include := range includes {
		err = ctx.runFile(ctx.base, path.Join("harness", include), vm)
		if err != nil {
			return early, err
		}
	}

	u := &url.URL{Scheme: "file", Path: "/" + path.Join(ctx.base, name)}
	base := u.JoinPath("..")
	mr := modules.NewModuleResolver(nil,
		func(specifier *url.URL, _ string) ([]byte, error) {
			b, err := fs.ReadFile(os.DirFS("."), specifier.Path[1:])

			if ctx.compatibilityMode == lib.CompatibilityModeExperimentalEnhanced {
				code, _, err := compiler.StripTypes(string(b), name)
				if err != nil {
					return nil, err
				}
				b = []byte(code)
			}
			return b, err
		},
		ctx.compiler(), base, usage.New(), testutils.NewLogger(ctx.t))

	ms := modules.NewModuleSystem(mr, moduleRuntime.VU)
	moduleRuntime.VU.InitEnvField.CWD = base

	early = false
	result, err := ms.RunSourceData(&loader.SourceData{
		Data: []byte(src),
		URL:  u,
	})
	if err == nil {
		var finished bool
		_, finished, err = result.Result()
		if !finished {
			panic("tc39 has no tests where this should happen")
		}
	}

	return early, err
}

func (ctx *tc39TestCtx) runTC39Tests(name string) {
	files, err := os.ReadDir(path.Join(ctx.base, name))
	if err != nil {
		ctx.t.Fatal(err)
	}

outer:
	for _, file := range files {
		if file.Name()[0] == '.' {
			continue
		}
		newName := path.Join(name, file.Name())
		for _, skipWord := range skipWords {
			if strings.Contains(newName, skipWord) {
				ctx.t.Run(newName, func(t *testing.T) {
					t.Skipf("Skip %s because %s is not supported", newName, skipWord)
				})
				continue outer
			}
		}
		for _, path := range pathBasedBlock { // TODO: use trie / binary search?
			if strings.HasPrefix(newName, path) {
				ctx.t.Run(newName, func(t *testing.T) {
					t.Skipf("Skip %s because of path based block", newName)
				})
				continue outer
			}
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

//nolint:paralleltest // this is on purpose so the test can wait for the subtest to finish
func TestTC39(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	runTestTC39(t, lib.CompatibilityModeExtended)
	runTestTC39(t, lib.CompatibilityModeExperimentalEnhanced)
}

func runTestTC39(t *testing.T, compatibilityMode lib.CompatibilityMode) {
	t.Helper()

	if _, err := os.Stat(tc39BASE); err != nil {
		t.Skipf("If you want to run tc39 tests, you need to run the 'checkout.sh` script in the directory to get  https://github.com/tc39/test262 at the correct last tested commit (%v)", err)
	}

	ctx := &tc39TestCtx{
		base:              tc39BASE,
		compatibilityMode: compatibilityMode,
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
	if len(ctx.errors) > 0 && *update {
		filename := breakingTestErrorsFilename(ctx.compatibilityMode)
		file, err := os.Create(filepath.Clean(filename))
		if err != nil {
			t.Logf("Error while creating %s: %s", filename, err)
		}

		enc := json.NewEncoder(file)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		err = enc.Encode(ctx.errors)
		if err != nil {
			t.Logf("Error while json encoding errors: %s", err)
		}
		err = file.Close()
		if err != nil {
			t.Logf("Error while closing %s: %s", filename, err)
		}
	}
}
