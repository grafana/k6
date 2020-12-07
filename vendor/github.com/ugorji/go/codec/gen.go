// +build codecgen.exec

// Copyright (c) 2012-2020 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode"
	"unicode/utf8"
)

// ---------------------------------------------------
// codecgen supports the full cycle of reflection-based codec:
//    - RawExt
//    - Raw
//    - Extensions
//    - (Binary|Text|JSON)(Unm|M)arshal
//    - generic by-kind
//
// This means that, for dynamic things, we MUST use reflection to at least get the reflect.Type.
// In those areas, we try to only do reflection or interface-conversion when NECESSARY:
//    - Extensions, only if Extensions are configured.
//
// However, codecgen doesn't support the following:
//   - Canonical option. (codecgen IGNORES it currently)
//     This is just because it has not been implemented.
//   - MissingFielder implementation.
//     If a type implements MissingFielder, it is completely ignored by codecgen.
//   - CheckCircularReference.
//     When encoding, if a circular reference is encountered, it will cause a stack overflow.
//
// During encode/decode, Selfer takes precedence.
// A type implementing Selfer will know how to encode/decode itself statically.
//
// The following field types are supported:
//     array: [n]T
//     slice: []T
//     map: map[K]V
//     primitive: [u]int[n], float(32|64), bool, string
//     struct
//
// ---------------------------------------------------
// Note that a Selfer cannot call (e|d).(En|De)code on itself,
// as this will cause a circular reference, as (En|De)code will call Selfer methods.
// Any type that implements Selfer must implement completely and not fallback to (En|De)code.
//
// In addition, code in this file manages the generation of fast-path implementations of
// encode/decode of slices/maps of primitive keys/values.
//
// Users MUST re-generate their implementations whenever the code shape changes.
// The generated code will panic if it was generated with a version older than the supporting library.
// ---------------------------------------------------
//
// codec framework is very feature rich.
// When encoding or decoding into an interface, it depends on the runtime type of the interface.
// The type of the interface may be a named type, an extension, etc.
// Consequently, we fallback to runtime codec for encoding/decoding interfaces.
// In addition, we fallback for any value which cannot be guaranteed at runtime.
// This allows us support ANY value, including any named types, specifically those which
// do not implement our interfaces (e.g. Selfer).
//
// This explains some slowness compared to other code generation codecs (e.g. msgp).
// This reduction in speed is only seen when your refers to interfaces,
// e.g. type T struct { A interface{}; B []interface{}; C map[string]interface{} }
//
// codecgen will panic if the file was generated with an old version of the library in use.
//
// Note:
//   It was a conscious decision to have gen.go always explicitly call EncodeNil or TryDecodeAsNil.
//   This way, there isn't a function call overhead just to see that we should not enter a block of code.
//
// Note:
//   codecgen-generated code depends on the variables defined by fast-path.generated.go.
//   consequently, you cannot run with tags "codecgen notfastpath".

// GenVersion is the current version of codecgen.
//
// MARKER: Increment this value each time codecgen changes fundamentally.
// Fundamental changes are:
//   - helper methods change (signature change, new ones added, some removed, etc)
//   - codecgen command line changes
//
// v1: Initial Version
// v2: -
// v3: Changes for Kubernetes:
//     changes in signature of some unpublished helper methods and codecgen cmdline arguments.
// v4: Removed separator support from (en|de)cDriver, and refactored codec(gen)
// v5: changes to support faster json decoding. Let encoder/decoder maintain state of collections.
// v6: removed unsafe from gen, and now uses codecgen.exec tag
// v7: -
// v8: current - we now maintain compatibility with old generated code.
// v9: skipped
// v10: modified encDriver and decDriver interfaces.
// v11: remove deprecated methods of encDriver and decDriver.
// v12: removed deprecated methods from genHelper and changed container tracking logic
// v13: 20190603 removed DecodeString - use DecodeStringAsBytes instead
// v14: 20190611 refactored nil handling: TryDecodeAsNil -> selective TryNil, etc
// v15: 20190626 encDriver.EncodeString handles StringToRaw flag inside handle
// v16: 20190629 refactoring for v1.1.6
// v17: 20200911 reduce number of types for which we generate fast path functions (v1.1.8)
// v18: 20201004 changed definition of genHelper...Extension (to take interface{}) and eliminated I2Rtid method
// v19: 20201115 updated codecgen cmdline flags and optimized output
// v20: 20201120 refactored GenHelper to one exported function
const genVersion = 20

const (
	genCodecPkg        = "codec1978" // MARKER: keep in sync with codecgen/gen.go
	genTempVarPfx      = "yy"
	genTopLevelVarName = "x"

	// ignore canBeNil parameter, and always set to true.
	// This is because nil can appear anywhere, so we should always check.
	genAnythingCanBeNil = true

	// if genUseOneFunctionForDecStructMap, make a single codecDecodeSelferFromMap function;
	// else make codecDecodeSelferFromMap{LenPrefix,CheckBreak} so that conditionals
	// are not executed a lot.
	//
	// From testing, it didn't make much difference in runtime, so keep as true (one function only)
	genUseOneFunctionForDecStructMap = true

	// genFastpathCanonical configures whether we support Canonical in fast path.
	// The savings is not much.
	//
	// MARKER: This MUST ALWAYS BE TRUE. fast-path.go.tmp doesn't handle it being false.
	genFastpathCanonical = true

	// genFastpathTrimTypes configures whether we trim uncommon fastpath types.
	genFastpathTrimTypes = true

	// genDecStructArrayInlineLoopCheck configures whether we create a next function
	// for each iteration in the loop and call it, or just inline it.
	//
	// with inlining, we get better performance but about 10% larger files.
	genDecStructArrayInlineLoopCheck = true
)

type genStructMapStyle uint8

const (
	genStructMapStyleConsolidated genStructMapStyle = iota
	genStructMapStyleLenPrefix
	genStructMapStyleCheckBreak
)

var (
	errGenAllTypesSamePkg        = errors.New("All types must be in the same package")
	errGenExpectArrayOrMap       = errors.New("unexpected type - expecting array/map/slice")
	errGenUnexpectedTypeFastpath = errors.New("fast-path: unexpected type - requires map or slice")

	genBase64enc  = base64.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789__")
	genQNameRegex = regexp.MustCompile(`[A-Za-z_.]+`)
)

type genBuf struct {
	buf []byte
}

func (x *genBuf) sIf(b bool, s, t string) *genBuf {
	if b {
		x.buf = append(x.buf, s...)
	} else {
		x.buf = append(x.buf, t...)
	}
	return x
}
func (x *genBuf) s(s string) *genBuf              { x.buf = append(x.buf, s...); return x }
func (x *genBuf) b(s []byte) *genBuf              { x.buf = append(x.buf, s...); return x }
func (x *genBuf) v() string                       { return string(x.buf) }
func (x *genBuf) f(s string, args ...interface{}) { x.s(fmt.Sprintf(s, args...)) }
func (x *genBuf) reset() {
	if x.buf != nil {
		x.buf = x.buf[:0]
	}
}

// genRunner holds some state used during a Gen run.
type genRunner struct {
	w io.Writer // output
	c uint64    // counter used for generating varsfx
	f uint64    // counter used for saying false

	t  []reflect.Type   // list of types to run selfer on
	tc reflect.Type     // currently running selfer on this type
	te map[uintptr]bool // types for which the encoder has been created
	td map[uintptr]bool // types for which the decoder has been created
	tz map[uintptr]bool // types for which GenIsZero has been created

	cp string // codec import path

	im  map[string]reflect.Type // imports to add
	imn map[string]string       // package names of imports to add
	imc uint64                  // counter for import numbers

	is map[reflect.Type]struct{} // types seen during import search
	bp string                    // base PkgPath, for which we are generating for

	cpfx string // codec package prefix

	ty map[reflect.Type]struct{} // types for which GenIsZero *should* be created
	tm map[reflect.Type]struct{} // types for which enc/dec must be generated
	ts []reflect.Type            // types for which enc/dec must be generated

	xs string // top level variable/constant suffix
	hn string // fn helper type name

	ti *TypeInfos
	// rr *rand.Rand // random generator for file-specific types

	jsonOnlyWhen, toArrayWhen, omitEmptyWhen *bool

	nx bool // no extensions
}

type genIfClause struct {
	hasIf bool
}

func (g *genIfClause) end(x *genRunner) {
	if g.hasIf {
		x.line("}")
	}
}

func (g *genIfClause) c(last bool) (v string) {
	if last {
		if g.hasIf {
			v = " } else { "
		}
	} else if g.hasIf {
		v = " } else if "
	} else {
		v = "if "
		g.hasIf = true
	}
	return
}

// Gen will write a complete go file containing Selfer implementations for each
// type passed. All the types must be in the same package.
//
// Library users: DO NOT USE IT DIRECTLY. IT WILL CHANGE CONTINUOUSLY WITHOUT NOTICE.
func Gen(w io.Writer, buildTags, pkgName, uid string, noExtensions bool,
	jsonOnlyWhen, toArrayWhen, omitEmptyWhen *bool,
	ti *TypeInfos, typ ...reflect.Type) {
	// All types passed to this method do not have a codec.Selfer method implemented directly.
	// codecgen already checks the AST and skips any types that define the codec.Selfer methods.
	// Consequently, there's no need to check and trim them if they implement codec.Selfer

	if len(typ) == 0 {
		return
	}
	x := genRunner{
		w:             w,
		t:             typ,
		te:            make(map[uintptr]bool),
		td:            make(map[uintptr]bool),
		tz:            make(map[uintptr]bool),
		im:            make(map[string]reflect.Type),
		imn:           make(map[string]string),
		is:            make(map[reflect.Type]struct{}),
		tm:            make(map[reflect.Type]struct{}),
		ty:            make(map[reflect.Type]struct{}),
		ts:            []reflect.Type{},
		bp:            genImportPath(typ[0]),
		xs:            uid,
		ti:            ti,
		jsonOnlyWhen:  jsonOnlyWhen,
		toArrayWhen:   toArrayWhen,
		omitEmptyWhen: omitEmptyWhen,

		nx: noExtensions,
	}
	if x.ti == nil {
		x.ti = defTypeInfos
	}
	if x.xs == "" {
		rr := rand.New(rand.NewSource(time.Now().UnixNano()))
		x.xs = strconv.FormatInt(rr.Int63n(9999), 10)
	}

	// gather imports first:
	x.cp = genImportPath(reflect.TypeOf(x))
	x.imn[x.cp] = genCodecPkg
	for _, t := range typ {
		// xdebugf("###########: PkgPath: '%v', Name: '%s'\n", genImportPath(t), t.Name())
		if genImportPath(t) != x.bp {
			halt.onerror(errGenAllTypesSamePkg)
		}
		x.genRefPkgs(t)
	}
	x.line("// +build go1.6")
	if buildTags != "" {
		x.line("// +build " + buildTags)
	}
	x.line(`

// Code generated by codecgen - DO NOT EDIT.

`)
	x.line("package " + pkgName)
	x.line("")
	x.line("import (")
	if x.cp != x.bp {
		x.cpfx = genCodecPkg + "."
		x.linef("%s \"%s\"", genCodecPkg, x.cp)
	}
	// use a sorted set of im keys, so that we can get consistent output
	imKeys := make([]string, 0, len(x.im))
	for k := range x.im {
		imKeys = append(imKeys, k)
	}
	sort.Strings(imKeys)
	for _, k := range imKeys { // for k, _ := range x.im {
		if k == x.imn[k] {
			x.linef("\"%s\"", k)
		} else {
			x.linef("%s \"%s\"", x.imn[k], k)
		}
	}
	// add required packages
	for _, k := range [...]string{"runtime", "errors", "strconv"} { // "reflect", "fmt"
		if _, ok := x.im[k]; !ok {
			x.line("\"" + k + "\"")
		}
	}
	x.line(")")
	x.line("")

	x.line("const (")
	x.linef("// ----- content types ----")
	x.linef("codecSelferCcUTF8%s = %v", x.xs, int64(cUTF8))
	x.linef("codecSelferCcRAW%s = %v", x.xs, int64(cRAW))
	x.linef("// ----- value types used ----")
	for _, vt := range [...]valueType{
		valueTypeArray, valueTypeMap, valueTypeString,
		valueTypeInt, valueTypeUint, valueTypeFloat,
		valueTypeNil,
	} {
		x.linef("codecSelferValueType%s%s = %v", vt.String(), x.xs, int64(vt))
	}

	x.linef("codecSelferBitsize%s = uint8(32 << (^uint(0) >> 63))", x.xs)
	x.linef("codecSelferDecContainerLenNil%s = %d", x.xs, int64(containerLenNil))
	x.line(")")
	x.line("var (")
	x.line("errCodecSelferOnlyMapOrArrayEncodeToStruct" + x.xs + " = " + "errors.New(`only encoded map or array can be decoded into a struct`)")
	x.line(")")
	x.line("")

	x.hn = "codecSelfer" + x.xs
	x.line("type " + x.hn + " struct{}")
	x.line("")
	x.linef("func %sFalse() bool { return false }", x.hn)
	x.linef("func %sTrue() bool { return true }", x.hn)
	x.line("")
	x.varsfxreset()
	x.line("func init() {")
	x.linef("if %sGenVersion != %v {", x.cpfx, genVersion)
	x.line("_, file, _, _ := runtime.Caller(0)")
	x.linef("ver := strconv.FormatInt(int64(%sGenVersion), 10)", x.cpfx)
	x.outf(`panic(errors.New("codecgen version mismatch: current: %v, need " + ver + ". Re-generate file: " + file))`, genVersion)
	x.linef("}")
	if len(imKeys) > 0 {
		x.line("if false { // reference the types, but skip this branch at build/run time")
		for _, k := range imKeys {
			t := x.im[k]
			x.linef("var _ %s.%s", x.imn[k], t.Name())
		}
		x.line("} ") // close if false
	}
	x.line("}") // close init
	x.line("")

	// generate rest of type info
	for _, t := range typ {
		x.tc = t
		x.selfer(true)
		x.selfer(false)
		x.tryGenIsZero(t)
	}

	for _, t := range x.ts {
		rtid := rt2id(t)
		// generate enc functions for all these slice/map types.
		x.varsfxreset()
		x.linef("func (x %s) enc%s(v %s%s, e *%sEncoder) {", x.hn, x.genMethodNameT(t), x.arr2str(t, "*"), x.genTypeName(t), x.cpfx)
		x.genRequiredMethodVars(true)
		switch t.Kind() {
		case reflect.Array, reflect.Slice, reflect.Chan:
			x.encListFallback("v", t)
		case reflect.Map:
			x.encMapFallback("v", t)
		default:
			halt.onerror(errGenExpectArrayOrMap)
		}
		x.line("}")
		x.line("")

		// generate dec functions for all these slice/map types.
		x.varsfxreset()
		x.linef("func (x %s) dec%s(v *%s, d *%sDecoder) {", x.hn, x.genMethodNameT(t), x.genTypeName(t), x.cpfx)
		x.genRequiredMethodVars(false)
		switch t.Kind() {
		case reflect.Array, reflect.Slice, reflect.Chan:
			x.decListFallback("v", rtid, t)
		case reflect.Map:
			x.decMapFallback("v", rtid, t)
		default:
			halt.onerror(errGenExpectArrayOrMap)
		}
		x.line("}")
		x.line("")
	}

	for t := range x.ty {
		x.tryGenIsZero(t)
		x.line("")
	}

	x.line("")
}

func (x *genRunner) checkForSelfer(t reflect.Type, varname string) bool {
	// return varname != genTopLevelVarName && t != x.tc
	// the only time we checkForSelfer is if we are not at the TOP of the generated code.
	return varname != genTopLevelVarName
}

func (x *genRunner) arr2str(t reflect.Type, s string) string {
	if t.Kind() == reflect.Array {
		return s
	}
	return ""
}

func (x *genRunner) genRequiredMethodVars(encode bool) {
	x.line("var h " + x.hn)
	if encode {
		x.line("z, r := " + x.cpfx + "GenHelper().Encoder(e)")
	} else {
		x.line("z, r := " + x.cpfx + "GenHelper().Decoder(d)")
	}
	x.line("_, _, _ = h, z, r")
}

func (x *genRunner) genRefPkgs(t reflect.Type) {
	if _, ok := x.is[t]; ok {
		return
	}
	x.is[t] = struct{}{}
	tpkg, tname := genImportPath(t), t.Name()
	if tpkg != "" && tpkg != x.bp && tpkg != x.cp && tname != "" && tname[0] >= 'A' && tname[0] <= 'Z' {
		if _, ok := x.im[tpkg]; !ok {
			x.im[tpkg] = t
			if idx := strings.LastIndex(tpkg, "/"); idx < 0 {
				x.imn[tpkg] = tpkg
			} else {
				x.imc++
				x.imn[tpkg] = "pkg" + strconv.FormatUint(x.imc, 10) + "_" + genGoIdentifier(tpkg[idx+1:], false)
			}
		}
	}
	switch t.Kind() {
	case reflect.Array, reflect.Slice, reflect.Ptr, reflect.Chan:
		x.genRefPkgs(t.Elem())
	case reflect.Map:
		x.genRefPkgs(t.Elem())
		x.genRefPkgs(t.Key())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if fname := t.Field(i).Name; fname != "" && fname[0] >= 'A' && fname[0] <= 'Z' {
				x.genRefPkgs(t.Field(i).Type)
			}
		}
	}
}

// sayFalse will either say "false" or use a function call that returns false.
func (x *genRunner) sayFalse() string {
	x.f++
	if x.f%2 == 0 {
		return x.hn + "False()"
	}
	return "false"
}

// sayFalse will either say "true" or use a function call that returns true.
func (x *genRunner) sayTrue() string {
	x.f++
	if x.f%2 == 0 {
		return x.hn + "True()"
	}
	return "true"
}

func (x *genRunner) varsfx() string {
	x.c++
	return strconv.FormatUint(x.c, 10)
}

func (x *genRunner) varsfxreset() {
	x.c = 0
}

func (x *genRunner) out(s string) {
	_, err := io.WriteString(x.w, s)
	genCheckErr(err)
}

func (x *genRunner) outf(s string, params ...interface{}) {
	_, err := fmt.Fprintf(x.w, s, params...)
	genCheckErr(err)
}

func (x *genRunner) line(s string) {
	x.out(s)
	if len(s) == 0 || s[len(s)-1] != '\n' {
		x.out("\n")
	}
}

func (x *genRunner) lineIf(s string) {
	if s != "" {
		x.line(s)
	}
}

func (x *genRunner) linef(s string, params ...interface{}) {
	x.outf(s, params...)
	if len(s) == 0 || s[len(s)-1] != '\n' {
		x.out("\n")
	}
}

func (x *genRunner) genTypeName(t reflect.Type) (n string) {
	// if the type has a PkgPath, which doesn't match the current package,
	// then include it.
	// We cannot depend on t.String() because it includes current package,
	// or t.PkgPath because it includes full import path,
	//
	var ptrPfx string
	for t.Kind() == reflect.Ptr {
		ptrPfx += "*"
		t = t.Elem()
	}
	if tn := t.Name(); tn != "" {
		return ptrPfx + x.genTypeNamePrim(t)
	}
	switch t.Kind() {
	case reflect.Map:
		return ptrPfx + "map[" + x.genTypeName(t.Key()) + "]" + x.genTypeName(t.Elem())
	case reflect.Slice:
		return ptrPfx + "[]" + x.genTypeName(t.Elem())
	case reflect.Array:
		return ptrPfx + "[" + strconv.FormatInt(int64(t.Len()), 10) + "]" + x.genTypeName(t.Elem())
	case reflect.Chan:
		return ptrPfx + t.ChanDir().String() + " " + x.genTypeName(t.Elem())
	default:
		if t == intfTyp {
			return ptrPfx + "interface{}"
		} else {
			return ptrPfx + x.genTypeNamePrim(t)
		}
	}
}

func (x *genRunner) genTypeNamePrim(t reflect.Type) (n string) {
	if t.Name() == "" {
		return t.String()
	} else if genImportPath(t) == "" || genImportPath(t) == genImportPath(x.tc) {
		return t.Name()
	} else {
		return x.imn[genImportPath(t)] + "." + t.Name()
		// return t.String() // best way to get the package name inclusive
	}
}

func (x *genRunner) genZeroValueR(t reflect.Type) string {
	// if t is a named type, w
	switch t.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Chan, reflect.Func,
		reflect.Slice, reflect.Map, reflect.Invalid:
		return "nil"
	case reflect.Bool:
		return "false"
	case reflect.String:
		return `""`
	case reflect.Struct, reflect.Array:
		return x.genTypeName(t) + "{}"
	default: // all numbers
		return "0"
	}
}

func (x *genRunner) genMethodNameT(t reflect.Type) (s string) {
	return genMethodNameT(t, x.tc)
}

func (x *genRunner) tryGenIsZero(t reflect.Type) (done bool) {
	if t.Kind() != reflect.Struct || t.Implements(isCodecEmptyerTyp) {
		return
	}

	rtid := rt2id(t)

	if _, ok := x.tz[rtid]; ok {
		delete(x.ty, t)
		return
	}

	x.tz[rtid] = true
	delete(x.ty, t)

	ti := x.ti.get(rtid, t)
	tisfi := ti.sfiSrc // always use sequence from file. decStruct expects same thing.
	varname := genTopLevelVarName

	x.linef("func (%s *%s) IsCodecEmpty() bool {", varname, x.genTypeName(t))

	anonSeen := make(map[reflect.Type]bool)
	var omitline genBuf
	for _, si := range tisfi {
		if si.path.parent != nil {
			root := si.path.root()
			if anonSeen[root.typ] {
				continue
			}
			anonSeen[root.typ] = true
		}
		t2 := genOmitEmptyLinePreChecks(varname, t, si, &omitline, true)
		// if Ptr, we already checked if nil above
		if t2.Type.Kind() != reflect.Ptr {
			x.doEncOmitEmptyLine(t2, varname, &omitline)
			omitline.s(" || ")
		}
	}
	omitline.s(" false")
	x.linef("return !(%s)", omitline.v())

	x.line("}")
	x.line("")
	return true
}

func (x *genRunner) selfer(encode bool) {
	t := x.tc
	t0 := t
	// always make decode use a pointer receiver,
	// and structs/arrays always use a ptr receiver (encode|decode)
	isptr := !encode || t.Kind() == reflect.Array || (t.Kind() == reflect.Struct && t != timeTyp)
	x.varsfxreset()

	fnSigPfx := "func (" + genTopLevelVarName + " "
	if isptr {
		fnSigPfx += "*"
	}
	fnSigPfx += x.genTypeName(t)
	x.out(fnSigPfx)

	if isptr {
		t = reflect.PtrTo(t)
	}
	if encode {
		x.line(") CodecEncodeSelf(e *" + x.cpfx + "Encoder) {")
		x.genRequiredMethodVars(true)
		x.encVar(genTopLevelVarName, t)
	} else {
		x.line(") CodecDecodeSelf(d *" + x.cpfx + "Decoder) {")
		x.genRequiredMethodVars(false)
		// do not use decVar, as there is no need to check TryDecodeAsNil
		// or way to elegantly handle that, and also setting it to a
		// non-nil value doesn't affect the pointer passed.
		// x.decVar(genTopLevelVarName, t, false)
		x.dec(genTopLevelVarName, t0, true)
	}
	x.line("}")
	x.line("")

	if encode || t0.Kind() != reflect.Struct {
		return
	}

	// write is containerMap
	if genUseOneFunctionForDecStructMap {
		x.out(fnSigPfx)
		x.line(") codecDecodeSelfFromMap(l int, d *" + x.cpfx + "Decoder) {")
		x.genRequiredMethodVars(false)
		x.decStructMap(genTopLevelVarName, "l", rt2id(t0), t0, genStructMapStyleConsolidated)
		x.line("}")
		x.line("")
	} else {
		x.out(fnSigPfx)
		x.line(") codecDecodeSelfFromMapLenPrefix(l int, d *" + x.cpfx + "Decoder) {")
		x.genRequiredMethodVars(false)
		x.decStructMap(genTopLevelVarName, "l", rt2id(t0), t0, genStructMapStyleLenPrefix)
		x.line("}")
		x.line("")

		x.out(fnSigPfx)
		x.line(") codecDecodeSelfFromMapCheckBreak(l int, d *" + x.cpfx + "Decoder) {")
		x.genRequiredMethodVars(false)
		x.decStructMap(genTopLevelVarName, "l", rt2id(t0), t0, genStructMapStyleCheckBreak)
		x.line("}")
		x.line("")
	}

	// write containerArray
	x.out(fnSigPfx)
	x.line(") codecDecodeSelfFromArray(l int, d *" + x.cpfx + "Decoder) {")
	x.genRequiredMethodVars(false)
	x.decStructArray(genTopLevelVarName, "l", "return", rt2id(t0), t0)
	x.line("}")
	x.line("")

}

// used for chan, array, slice, map
func (x *genRunner) xtraSM(varname string, t reflect.Type, encode, isptr bool) {
	var ptrPfx, addrPfx string
	if isptr {
		ptrPfx = "*"
	} else {
		addrPfx = "&"
	}
	if encode {
		x.linef("h.enc%s((%s%s)(%s), e)", x.genMethodNameT(t), ptrPfx, x.genTypeName(t), varname)
	} else {
		x.linef("h.dec%s((*%s)(%s%s), d)", x.genMethodNameT(t), x.genTypeName(t), addrPfx, varname)
	}
	x.registerXtraT(t)
}

func (x *genRunner) registerXtraT(t reflect.Type) {
	// recursively register the types
	if _, ok := x.tm[t]; ok {
		return
	}
	var tkey reflect.Type
	switch t.Kind() {
	case reflect.Chan, reflect.Slice, reflect.Array:
	case reflect.Map:
		tkey = t.Key()
	default:
		return
	}
	x.tm[t] = struct{}{}
	x.ts = append(x.ts, t)
	// check if this refers to any xtra types eg. a slice of array: add the array
	x.registerXtraT(t.Elem())
	if tkey != nil {
		x.registerXtraT(tkey)
	}
}

// encVar will encode a variable.
// The parameter, t, is the reflect.Type of the variable itself
func (x *genRunner) encVar(varname string, t reflect.Type) {
	var checkNil bool
	// case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan:
	// do not include checkNil for slice and maps, as we already checkNil below it
	switch t.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Chan:
		checkNil = true
	}
	x.encVarChkNil(varname, t, checkNil)
}

func (x *genRunner) encVarChkNil(varname string, t reflect.Type, checkNil bool) {
	if checkNil {
		x.linef("if %s == nil { r.EncodeNil() } else {", varname)
	}

	switch t.Kind() {
	case reflect.Ptr:
		telem := t.Elem()
		tek := telem.Kind()
		if tek == reflect.Array || (tek == reflect.Struct && telem != timeTyp) {
			x.enc(varname, genNonPtr(t), true)
			break
		}
		i := x.varsfx()
		x.line(genTempVarPfx + i + " := *" + varname)
		x.enc(genTempVarPfx+i, genNonPtr(t), false)
	case reflect.Struct, reflect.Array:
		if t == timeTyp {
			x.enc(varname, t, false)
			break
		}
		i := x.varsfx()
		x.line(genTempVarPfx + i + " := &" + varname)
		x.enc(genTempVarPfx+i, t, true)
	default:
		x.enc(varname, t, false)
	}

	if checkNil {
		x.line("}")
	}
}

// enc will encode a variable (varname) of type t, where t represents T.
// if t is !time.Time and t is of kind reflect.Struct or reflect.Array, varname is of type *T
// (to prevent copying),
// else t is of type T
func (x *genRunner) enc(varname string, t reflect.Type, isptr bool) {
	rtid := rt2id(t)
	ti2 := x.ti.get(rtid, t)
	// We call CodecEncodeSelf if one of the following are honored:
	//   - the type already implements Selfer, call that
	//   - the type has a Selfer implementation just created, use that
	//   - the type is in the list of the ones we will generate for, but it is not currently being generated

	mi := x.varsfx()
	// tptr := reflect.PtrTo(t)
	// tk := t.Kind()

	// check if
	//   - type is time.Time, RawExt, Raw
	//   - the type implements (Text|JSON|Binary)(Unm|M)arshal

	var hasIf genIfClause
	defer hasIf.end(x) // end if block (if necessary)

	var ptrPfx, addrPfx string
	if isptr {
		ptrPfx = "*"
	} else {
		addrPfx = "&"
	}

	if t == timeTyp {
		x.linef("%s !z.EncBasicHandle().TimeNotBuiltin { r.EncodeTime(%s%s)", hasIf.c(false), ptrPfx, varname)
		// return
	}
	if t == rawTyp {
		x.linef("%s z.EncRaw(%s%s)", hasIf.c(true), ptrPfx, varname)
		return
	}
	if t == rawExtTyp {
		x.linef("%s r.EncodeRawExt(%s%s)", hasIf.c(true), addrPfx, varname)
		return
	}
	// only check for extensions if extensions are configured,
	// and the type is named, and has a packagePath,
	// and this is not the CodecEncodeSelf or CodecDecodeSelf method (i.e. it is not a Selfer)
	if !x.nx && varname != genTopLevelVarName && genImportPath(t) != "" && t.Name() != "" {
		yy := fmt.Sprintf("%sxt%s", genTempVarPfx, mi)
		x.linef("%s %s := z.Extension(%s); %s != nil { z.EncExtension(%s, %s) ",
			hasIf.c(false), yy, varname, yy, varname, yy)
	}

	if x.checkForSelfer(t, varname) {
		if ti2.flagSelfer {
			x.linef("%s %s.CodecEncodeSelf(e)", hasIf.c(true), varname)
			return
		} else if ti2.flagSelferPtr {
			x.linef("%s %ssf%s := &%s", hasIf.c(true), genTempVarPfx, mi, varname)
			x.linef("%ssf%s.CodecEncodeSelf(e)", genTempVarPfx, mi)
			return
		}

		if _, ok := x.te[rtid]; ok {
			x.linef("%s %s.CodecEncodeSelf(e)", hasIf.c(true), varname)
			return
		}
	}

	inlist := false
	for _, t0 := range x.t {
		if t == t0 {
			inlist = true
			if x.checkForSelfer(t, varname) {
				x.line(varname + ".CodecEncodeSelf(e)")
				return
			}
			break
		}
	}

	var rtidAdded bool
	if t == x.tc {
		x.te[rtid] = true
		rtidAdded = true
	}

	if ti2.flagBinaryMarshaler {
		x.linef("%s z.EncBinary() { z.EncBinaryMarshal(%s%v) ", hasIf.c(false), ptrPfx, varname)
	} else if ti2.flagBinaryMarshalerPtr {
		x.linef("%s z.EncBinary() { z.EncBinaryMarshal(%s%v) ", hasIf.c(false), addrPfx, varname)
	}

	if ti2.flagJsonMarshaler {
		x.linef("%s !z.EncBinary() && z.IsJSONHandle() { z.EncJSONMarshal(%s%v) ", hasIf.c(false), ptrPfx, varname)
	} else if ti2.flagJsonMarshalerPtr {
		x.linef("%s !z.EncBinary() && z.IsJSONHandle() { z.EncJSONMarshal(%s%v) ", hasIf.c(false), addrPfx, varname)
	} else if ti2.flagTextMarshaler {
		x.linef("%s !z.EncBinary() { z.EncTextMarshal(%s%v) ", hasIf.c(false), ptrPfx, varname)
	} else if ti2.flagTextMarshalerPtr {
		x.linef("%s !z.EncBinary() { z.EncTextMarshal(%s%v) ", hasIf.c(false), addrPfx, varname)
	}

	x.lineIf(hasIf.c(true))

	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		x.line("r.EncodeInt(int64(" + varname + "))")
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		x.line("r.EncodeUint(uint64(" + varname + "))")
	case reflect.Float32:
		x.line("r.EncodeFloat32(float32(" + varname + "))")
	case reflect.Float64:
		x.line("r.EncodeFloat64(float64(" + varname + "))")
	case reflect.Bool:
		x.line("r.EncodeBool(bool(" + varname + "))")
	case reflect.String:
		x.linef("r.EncodeString(string(%s))", varname)
	case reflect.Chan:
		x.xtraSM(varname, t, true, false)
		// x.encListFallback(varname, rtid, t)
	case reflect.Array:
		x.xtraSM(varname, t, true, true)
	case reflect.Slice:
		// if nil, call dedicated function
		// if a []byte, call dedicated function
		// if a known fastpath slice, call dedicated function
		// else write encode function in-line.
		// - if elements are primitives or Selfers, call dedicated function on each member.
		// - else call Encoder.encode(XXX) on it.
		x.linef("if %s == nil { r.EncodeNil() } else {", varname)
		if rtid == uint8SliceTypId {
			x.line("r.EncodeStringBytesRaw([]byte(" + varname + "))")
		} else if fastpathAvIndex(rtid) != -1 {
			g := x.newFastpathGenV(t)
			x.line("z.F." + g.MethodNamePfx("Enc", false) + "V(" + varname + ", e)")
		} else {
			x.xtraSM(varname, t, true, false)
		}
		x.linef("} // end block: if %s slice == nil", varname)
	case reflect.Map:
		// if nil, call dedicated function
		// if a known fastpath map, call dedicated function
		// else write encode function in-line.
		// - if elements are primitives or Selfers, call dedicated function on each member.
		// - else call Encoder.encode(XXX) on it.
		x.linef("if %s == nil { r.EncodeNil() } else {", varname)
		if fastpathAvIndex(rtid) != -1 {
			g := x.newFastpathGenV(t)
			x.line("z.F." + g.MethodNamePfx("Enc", false) + "V(" + varname + ", e)")
		} else {
			x.xtraSM(varname, t, true, false)
		}
		x.linef("} // end block: if %s map == nil", varname)
	case reflect.Struct:
		if !inlist {
			delete(x.te, rtid)
			x.line("z.EncFallback(" + varname + ")")
			break
		}
		x.encStruct(varname, rtid, t)
	default:
		if rtidAdded {
			delete(x.te, rtid)
		}
		x.line("z.EncFallback(" + varname + ")")
	}
}

func (x *genRunner) encZero(t reflect.Type) {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		x.line("r.EncodeInt(0)")
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		x.line("r.EncodeUint(0)")
	case reflect.Float32:
		x.line("r.EncodeFloat32(0)")
	case reflect.Float64:
		x.line("r.EncodeFloat64(0)")
	case reflect.Bool:
		x.line("r.EncodeBool(false)")
	case reflect.String:
		x.linef(`r.EncodeString("")`)
	default:
		x.line("r.EncodeNil()")
	}
}

func genOmitEmptyLinePreChecks(varname string, t reflect.Type, si *structFieldInfo, omitline *genBuf, oneLevel bool) (t2 reflect.StructField) {
	// xdebug2f("calling genOmitEmptyLinePreChecks on: %v", t)
	t2typ := t
	varname3 := varname
	// go through the loop, record the t2 field explicitly,
	// and gather the omit line if embedded in pointers.
	fullpath := si.path.fullpath()
	for i, path := range fullpath {
		for t2typ.Kind() == reflect.Ptr {
			t2typ = t2typ.Elem()
		}
		t2 = t2typ.Field(int(path.index))
		t2typ = t2.Type
		varname3 = varname3 + "." + t2.Name
		// do not include actual field in the omit line.
		// that is done subsequently (right after - below).
		if i+1 < len(fullpath) && t2typ.Kind() == reflect.Ptr {
			omitline.s(varname3).s(" != nil && ")
		}
		if oneLevel {
			break
		}
	}
	return
}

func (x *genRunner) doEncOmitEmptyLine(t2 reflect.StructField, varname string, buf *genBuf) {
	x.f = 0
	x.encOmitEmptyLine(t2, varname, buf)
}

func (x *genRunner) encOmitEmptyLine(t2 reflect.StructField, varname string, buf *genBuf) {
	// xdebugf("calling encOmitEmptyLine on: %v", t2.Type)
	// smartly check omitEmpty on a struct type, as it may contain uncomparable map/slice/etc.
	// also, for maps/slices/arrays, check if len ! 0 (not if == zero value)
	varname2 := varname + "." + t2.Name
	switch t2.Type.Kind() {
	case reflect.Struct:
		rtid2 := rt2id(t2.Type)
		ti2 := x.ti.get(rtid2, t2.Type)
		// xdebugf(">>>> structfield: omitempty: type: %s, field: %s\n", t2.Type.Name(), t2.Name)
		if ti2.rtid == timeTypId {
			buf.s("!(").s(varname2).s(".IsZero())")
			break
		}
		if ti2.flagIsZeroerPtr || ti2.flagIsZeroer {
			buf.s("!(").s(varname2).s(".IsZero())")
			break
		}
		if t2.Type.Implements(isCodecEmptyerTyp) {
			buf.s("!(").s(varname2).s(".IsCodecEmpty())")
			break
		}
		_, ok := x.tz[rtid2]
		if ok {
			buf.s("!(").s(varname2).s(".IsCodecEmpty())")
			break
		}
		// if we *should* create a IsCodecEmpty for it, but haven't yet, add it here
		// _, ok = x.ty[rtid2]
		if genImportPath(t2.Type) == x.bp {
			x.ty[t2.Type] = struct{}{}
			buf.s("!(").s(varname2).s(".IsCodecEmpty())")
			break
		}
		if ti2.flagComparable {
			buf.s(varname2).s(" != ").s(x.genZeroValueR(t2.Type))
			break
		}
		// MARKER: we shouldn't get here at all ...
		fmt.Printf("???? !!!! We shouldn't get to this point !!!! ????")
		// buf.s("(")
		buf.s(x.sayFalse()) // buf.s("false")
		for i, n := 0, t2.Type.NumField(); i < n; i++ {
			f := t2.Type.Field(i)
			if f.PkgPath != "" { // unexported
				continue
			}
			buf.s(" || ")
			x.encOmitEmptyLine(f, varname2, buf)
		}
		//buf.s(")")
	case reflect.Bool:
		buf.s("bool(").s(varname2).s(")")
	case reflect.Map, reflect.Slice, reflect.Array, reflect.Chan:
		buf.s("len(").s(varname2).s(") != 0")
	default:
		buf.s(varname2).s(" != ").s(x.genZeroValueR(t2.Type))
	}
}

func (x *genRunner) encStruct(varname string, rtid uintptr, t reflect.Type) {
	// Use knowledge from structfieldinfo (mbs, encodable fields. Ignore omitempty. )
	// replicate code in kStruct i.e. for each field, deref type to non-pointer, and call x.enc on it

	// if t === type currently running selfer on, do for all
	ti := x.ti.get(rtid, t)
	i := x.varsfx()
	// sepVarname := genTempVarPfx + "sep" + i
	numfieldsvar := genTempVarPfx + "q" + i
	ti2arrayvar := genTempVarPfx + "r" + i
	struct2arrvar := genTempVarPfx + "2arr" + i

	tisfi := ti.sfiSrc // always use sequence from file. decStruct expects same thing.

	type genFQN struct {
		i       string
		fqname  string
		nilLine genBuf
		nilVar  string
		canNil  bool
		sf      reflect.StructField
	}

	genFQNs := make([]genFQN, len(tisfi))
	for j, si := range tisfi {
		q := &genFQNs[j]
		q.i = x.varsfx()
		q.nilVar = genTempVarPfx + "n" + q.i
		q.canNil = false
		q.fqname = varname
		{
			t2typ := t
			fullpath := si.path.fullpath()
			for _, path := range fullpath {
				for t2typ.Kind() == reflect.Ptr {
					t2typ = t2typ.Elem()
				}
				q.sf = t2typ.Field(int(path.index))
				t2typ = q.sf.Type
				q.fqname += "." + q.sf.Name
				if t2typ.Kind() == reflect.Ptr {
					if !q.canNil {
						q.nilLine.f("%s == nil", q.fqname)
						q.canNil = true
					} else {
						q.nilLine.f(" || %s == nil", q.fqname)
					}
				}
			}
		}
	}

	// x.line(sepVarname + " := !z.EncBinary()")
	x.linef("%s := z.EncBasicHandle().StructToArray", struct2arrvar)
	// x.linef("_, _ = %s, %s", sepVarname, struct2arrvar)
	x.linef("_ = %s", struct2arrvar)
	x.linef("const %s bool = %v // struct tag has 'toArray'", ti2arrayvar, ti.toArray)

	for j := range genFQNs {
		q := &genFQNs[j]
		if q.canNil {
			x.linef("var %s bool = %s", q.nilVar, q.nilLine.v())
		}
	}

	// var nn int
	// due to omitEmpty, we need to calculate the
	// number of non-empty things we write out first.
	// This is required as we need to pre-determine the size of the container,
	// to support length-prefixing.
	omitEmptySometimes := x.omitEmptyWhen == nil
	omitEmptyAlways := (x.omitEmptyWhen != nil && *(x.omitEmptyWhen))
	// omitEmptyNever := (x.omitEmptyWhen != nil && !*(x.omitEmptyWhen))

	toArraySometimes := x.toArrayWhen == nil
	toArrayAlways := (x.toArrayWhen != nil && *(x.toArrayWhen))
	toArrayNever := (x.toArrayWhen != nil && !(*(x.toArrayWhen)))

	if (omitEmptySometimes && ti.anyOmitEmpty) || omitEmptyAlways {
		x.linef("var %s = [%v]bool{ // should field at this index be written?", numfieldsvar, len(tisfi))

		for _, si := range tisfi {
			if omitEmptySometimes && !si.path.omitEmpty {
				x.linef("true, // %s", si.encName) // si.fieldName)
				continue
			}
			var omitline genBuf
			t2 := genOmitEmptyLinePreChecks(varname, t, si, &omitline, false)
			x.doEncOmitEmptyLine(t2, varname, &omitline)
			x.linef("%s, // %s", omitline.v(), si.encName) // si.fieldName)
		}
		x.line("}")
		x.linef("_ = %s", numfieldsvar)
	}

	if toArraySometimes {
		x.linef("if %s || %s {", ti2arrayvar, struct2arrvar) // if ti.toArray
	}
	if toArraySometimes || toArrayAlways {
		x.linef("z.EncWriteArrayStart(%d)", len(tisfi))

		for j, si := range tisfi {
			doOmitEmptyCheck := (omitEmptySometimes && si.path.omitEmpty) || omitEmptyAlways
			q := &genFQNs[j]
			// if the type of the field is a Selfer, or one of the ones
			if q.canNil {
				x.linef("if %s { z.EncWriteArrayElem(); r.EncodeNil() } else { ", q.nilVar)
			}
			x.linef("z.EncWriteArrayElem()")
			if doOmitEmptyCheck {
				x.linef("if %s[%v] {", numfieldsvar, j)
			}
			x.encVarChkNil(q.fqname, q.sf.Type, false)
			if doOmitEmptyCheck {
				x.linef("} else {")
				x.encZero(q.sf.Type)
				x.linef("}")
			}
			if q.canNil {
				x.line("}")
			}
		}

		x.line("z.EncWriteArrayEnd()")
	}
	if toArraySometimes {
		x.linef("} else {") // if not ti.toArray
	}
	if toArraySometimes || toArrayNever {
		if (omitEmptySometimes && ti.anyOmitEmpty) || omitEmptyAlways {
			x.linef("var %snn%s int", genTempVarPfx, i)
			x.linef("for _, b := range %s { if b { %snn%s++ } }", numfieldsvar, genTempVarPfx, i)
			x.linef("z.EncWriteMapStart(%snn%s)", genTempVarPfx, i)
			x.linef("%snn%s = %v", genTempVarPfx, i, 0)
		} else {
			x.linef("z.EncWriteMapStart(%d)", len(tisfi))
		}

		for j, si := range tisfi {
			q := &genFQNs[j]
			doOmitEmptyCheck := (omitEmptySometimes && si.path.omitEmpty) || omitEmptyAlways
			if doOmitEmptyCheck {
				x.linef("if %s[%v] {", numfieldsvar, j)
			}
			x.linef("z.EncWriteMapElemKey()")

			// emulate EncStructFieldKey
			switch ti.keyType {
			case valueTypeInt:
				x.linef("r.EncodeInt(z.M.Int(strconv.ParseInt(`%s`, 10, 64)))", si.encName)
			case valueTypeUint:
				x.linef("r.EncodeUint(z.M.Uint(strconv.ParseUint(`%s`, 10, 64)))", si.encName)
			case valueTypeFloat:
				x.linef("r.EncodeFloat64(z.M.Float(strconv.ParseFloat(`%s`, 64)))", si.encName)
			default: // string
				if x.jsonOnlyWhen == nil {
					if si.path.encNameAsciiAlphaNum {
						x.linef(`if z.IsJSONHandle() { z.WriteStr("\"%s\"") } else { `, si.encName)
					}
					x.linef("r.EncodeString(`%s`)", si.encName)
					if si.path.encNameAsciiAlphaNum {
						x.linef("}")
					}
				} else if *(x.jsonOnlyWhen) {
					if si.path.encNameAsciiAlphaNum {
						x.linef(`z.WriteStr("\"%s\"")`, si.encName)
					} else {
						x.linef("r.EncodeString(`%s`)", si.encName)
					}
				} else {
					x.linef("r.EncodeString(`%s`)", si.encName)
				}
			}
			x.line("z.EncWriteMapElemValue()")
			if q.canNil {
				x.line("if " + q.nilVar + " { r.EncodeNil() } else { ")
				x.encVarChkNil(q.fqname, q.sf.Type, false)
				x.line("}")
			} else {
				x.encVarChkNil(q.fqname, q.sf.Type, false)
			}
			if doOmitEmptyCheck {
				x.line("}")
			}
		}
		x.line("z.EncWriteMapEnd()")
	}
	if toArraySometimes {
		x.linef("} ") // end if/else ti.toArray
	}
}

func (x *genRunner) encListFallback(varname string, t reflect.Type) {
	x.linef("if %s == nil { r.EncodeNil(); return }", varname)
	elemBytes := t.Elem().Kind() == reflect.Uint8
	if t.AssignableTo(uint8SliceTyp) {
		x.linef("r.EncodeStringBytesRaw([]byte(%s))", varname)
		return
	}
	if t.Kind() == reflect.Array && elemBytes {
		x.linef("r.EncodeStringBytesRaw(((*[%d]byte)(%s))[:])", t.Len(), varname)
		return
	}
	i := x.varsfx()
	if t.Kind() == reflect.Chan {
		type ts struct {
			Label, Chan, Slice, Sfx string
		}
		tm, err := template.New("").Parse(genEncChanTmpl)
		genCheckErr(err)
		x.linef("if %s == nil { r.EncodeNil() } else { ", varname)
		x.linef("var sch%s []%s", i, x.genTypeName(t.Elem()))
		err = tm.Execute(x.w, &ts{"Lsch" + i, varname, "sch" + i, i})
		genCheckErr(err)
		if elemBytes {
			x.linef("r.EncodeStringBytesRaw([]byte(%s))", "sch"+i)
			x.line("}")
			return
		}
		varname = "sch" + i
	}

	x.line("z.EncWriteArrayStart(len(" + varname + "))")
	x.linef("for _, %sv%s := range %s {", genTempVarPfx, i, varname)
	x.linef("z.EncWriteArrayElem()")

	x.encVar(genTempVarPfx+"v"+i, t.Elem())
	x.line("}")
	x.line("z.EncWriteArrayEnd()")
	if t.Kind() == reflect.Chan {
		x.line("}")
	}
}

func (x *genRunner) encMapFallback(varname string, t reflect.Type) {
	x.linef("if %s == nil { r.EncodeNil(); return }", varname)
	// MARKER: Canonical Option is not honored
	i := x.varsfx()
	x.line("z.EncWriteMapStart(len(" + varname + "))")
	x.linef("for %sk%s, %sv%s := range %s {", genTempVarPfx, i, genTempVarPfx, i, varname)
	x.linef("z.EncWriteMapElemKey()")
	x.encVar(genTempVarPfx+"k"+i, t.Key())
	x.line("z.EncWriteMapElemValue()")
	x.encVar(genTempVarPfx+"v"+i, t.Elem())
	x.line("}")
	x.line("z.EncWriteMapEnd()")
}

func (x *genRunner) decVarInitPtr(varname, nilvar string, t reflect.Type, si *structFieldInfo,
	newbuf, nilbuf *genBuf) (varname3 string, t2 reflect.StructField) {
	//we must accommodate anonymous fields, where the embedded field is a nil pointer in the value.
	// t2 = t.FieldByIndex(si.is)
	varname3 = varname
	t2typ := t
	t2kind := t2typ.Kind()
	var nilbufed bool
	if si != nil {
		fullpath := si.path.fullpath()
		for _, path := range fullpath {
			// only one-level pointers can be seen in a type
			if t2typ.Kind() == reflect.Ptr {
				t2typ = t2typ.Elem()
			}
			t2 = t2typ.Field(int(path.index))
			t2typ = t2.Type
			varname3 = varname3 + "." + t2.Name
			t2kind = t2typ.Kind()
			if t2kind != reflect.Ptr {
				continue
			}
			if newbuf != nil {
				if len(newbuf.buf) > 0 {
					newbuf.s("\n")
				}
				newbuf.f("if %s == nil { %s = new(%s) }", varname3, varname3, x.genTypeName(t2typ.Elem()))
			}
			if nilbuf != nil {
				if !nilbufed {
					nilbuf.s("if ").s(varname3).s(" != nil")
					nilbufed = true
				} else {
					nilbuf.s(" && ").s(varname3).s(" != nil")
				}
			}
		}
	}
	if nilbuf != nil {
		if nilbufed {
			nilbuf.s(" { ").s("// remove the if-true\n")
		}
		if nilvar != "" {
			nilbuf.s(nilvar).s(" = true")
		} else if tk := t2typ.Kind(); tk == reflect.Ptr {
			if strings.IndexByte(varname3, '.') != -1 || strings.IndexByte(varname3, '[') != -1 {
				nilbuf.s(varname3).s(" = nil")
			} else {
				nilbuf.s("*").s(varname3).s(" = ").s(x.genZeroValueR(t2typ.Elem()))
			}
		} else {
			nilbuf.s(varname3).s(" = ").s(x.genZeroValueR(t2typ))
		}
		if nilbufed {
			nilbuf.s("}")
		}
	}
	return
}

// decVar takes a variable called varname, of type t
func (x *genRunner) decVarMain(varname, rand string, t reflect.Type, checkNotNil bool) {
	// We only encode as nil if a nillable value.
	// This removes some of the wasted checks for TryDecodeAsNil.
	// We need to think about this more, to see what happens if omitempty, etc
	// cause a nil value to be stored when something is expected.
	// This could happen when decoding from a struct encoded as an array.
	// For that, decVar should be called with canNil=true, to force true as its value.
	var varname2 string
	if t.Kind() != reflect.Ptr {
		if t.PkgPath() != "" || !x.decTryAssignPrimitive(varname, t, false) {
			x.dec(varname, t, false)
		}
	} else {
		if checkNotNil {
			x.linef("if %s == nil { %s = new(%s) }", varname, varname, x.genTypeName(t.Elem()))
		}
		// Ensure we set underlying ptr to a non-nil value (so we can deref to it later).
		// There's a chance of a **T in here which is nil.
		var ptrPfx string
		for t = t.Elem(); t.Kind() == reflect.Ptr; t = t.Elem() {
			ptrPfx += "*"
			if checkNotNil {
				x.linef("if %s%s == nil { %s%s = new(%s)}", ptrPfx, varname, ptrPfx, varname, x.genTypeName(t))
			}
		}
		// Should we create temp var if a slice/map indexing? No. dec(...) can now handle it.

		if ptrPfx == "" {
			x.dec(varname, t, true)
		} else {
			varname2 = genTempVarPfx + "z" + rand
			x.line(varname2 + " := " + ptrPfx + varname)
			x.dec(varname2, t, true)
		}
	}
}

// decVar takes a variable called varname, of type t
func (x *genRunner) decVar(varname, nilvar string, t reflect.Type, canBeNil, checkNotNil bool) {

	// We only encode as nil if a nillable value.
	// This removes some of the wasted checks for TryDecodeAsNil.
	// We need to think about this more, to see what happens if omitempty, etc
	// cause a nil value to be stored when something is expected.
	// This could happen when decoding from a struct encoded as an array.
	// For that, decVar should be called with canNil=true, to force true as its value.

	i := x.varsfx()
	if t.Kind() == reflect.Ptr {
		var buf genBuf
		x.decVarInitPtr(varname, nilvar, t, nil, nil, &buf)
		x.linef("if r.TryNil() { %s } else {", buf.buf)
		x.decVarMain(varname, i, t, checkNotNil)
		x.line("} ")
	} else {
		x.decVarMain(varname, i, t, checkNotNil)
	}
}

// dec will decode a variable (varname) of type t or ptrTo(t) if isptr==true.
func (x *genRunner) dec(varname string, t reflect.Type, isptr bool) {
	// assumptions:
	//   - the varname is to a pointer already. No need to take address of it
	//   - t is always a baseType T (not a *T, etc).
	rtid := rt2id(t)
	ti2 := x.ti.get(rtid, t)

	// check if
	//   - type is time.Time, Raw, RawExt
	//   - the type implements (Text|JSON|Binary)(Unm|M)arshal

	mi := x.varsfx()

	var hasIf genIfClause
	defer hasIf.end(x)

	var ptrPfx, addrPfx string
	if isptr {
		ptrPfx = "*"
	} else {
		addrPfx = "&"
	}
	if t == timeTyp {
		x.linef("%s !z.DecBasicHandle().TimeNotBuiltin { %s%v = r.DecodeTime()", hasIf.c(false), ptrPfx, varname)
		// return
	}
	if t == rawTyp {
		x.linef("%s %s%v = z.DecRaw()", hasIf.c(true), ptrPfx, varname)
		return
	}

	if t == rawExtTyp {
		x.linef("%s r.DecodeExt(%s%v, 0, nil)", hasIf.c(true), addrPfx, varname)
		return
	}

	// only check for extensions if extensions are configured,
	// and the type is named, and has a packagePath,
	// and this is not the CodecEncodeSelf or CodecDecodeSelf method (i.e. it is not a Selfer)
	// xdebugf("genRunner.dec: varname: %v, t: %v, genImportPath: %v, t.Name: %v", varname, t, genImportPath(t), t.Name())
	if !x.nx && varname != genTopLevelVarName && genImportPath(t) != "" && t.Name() != "" {
		// first check if extensions are configued, before doing the interface conversion
		yy := fmt.Sprintf("%sxt%s", genTempVarPfx, mi)
		x.linef("%s %s := z.Extension(%s); %s != nil { z.DecExtension(%s%s, %s) ", hasIf.c(false), yy, varname, yy, addrPfx, varname, yy)
	}

	if x.checkForSelfer(t, varname) {
		if ti2.flagSelfer {
			x.linef("%s %s.CodecDecodeSelf(d)", hasIf.c(true), varname)
			return
		}
		if ti2.flagSelferPtr {
			x.linef("%s %s.CodecDecodeSelf(d)", hasIf.c(true), varname)
			return
		}
		if _, ok := x.td[rtid]; ok {
			x.linef("%s %s.CodecDecodeSelf(d)", hasIf.c(true), varname)
			return
		}
	}

	inlist := false
	for _, t0 := range x.t {
		if t == t0 {
			inlist = true
			if x.checkForSelfer(t, varname) {
				x.linef("%s %s.CodecDecodeSelf(d)", hasIf.c(true), varname)
				return
			}
			break
		}
	}

	var rtidAdded bool
	if t == x.tc {
		x.td[rtid] = true
		rtidAdded = true
	}

	if ti2.flagBinaryUnmarshaler {
		x.linef("%s z.DecBinary() { z.DecBinaryUnmarshal(%s%v) ", hasIf.c(false), ptrPfx, varname)
	} else if ti2.flagBinaryUnmarshalerPtr {
		x.linef("%s z.DecBinary() { z.DecBinaryUnmarshal(%s%v) ", hasIf.c(false), addrPfx, varname)
	}
	if ti2.flagJsonUnmarshaler {
		x.linef("%s !z.DecBinary() && z.IsJSONHandle() { z.DecJSONUnmarshal(%s%v)", hasIf.c(false), ptrPfx, varname)
	} else if ti2.flagJsonUnmarshalerPtr {
		x.linef("%s !z.DecBinary() && z.IsJSONHandle() { z.DecJSONUnmarshal(%s%v)", hasIf.c(false), addrPfx, varname)
	} else if ti2.flagTextUnmarshaler {
		x.linef("%s !z.DecBinary() { z.DecTextUnmarshal(%s%v)", hasIf.c(false), ptrPfx, varname)
	} else if ti2.flagTextUnmarshalerPtr {
		x.linef("%s !z.DecBinary() { z.DecTextUnmarshal(%s%v)", hasIf.c(false), addrPfx, varname)
	}

	x.lineIf(hasIf.c(true))

	if x.decTryAssignPrimitive(varname, t, isptr) {
		return
	}

	switch t.Kind() {
	case reflect.Array, reflect.Chan:
		x.xtraSM(varname, t, false, isptr)
	case reflect.Slice:
		// if a []byte, call dedicated function
		// if a known fastpath slice, call dedicated function
		// else write encode function in-line.
		// - if elements are primitives or Selfers, call dedicated function on each member.
		// - else call Encoder.encode(XXX) on it.
		if rtid == uint8SliceTypId {
			x.linef("%s%s = r.DecodeBytes(%s(%s[]byte)(%s), false)",
				ptrPfx, varname, ptrPfx, ptrPfx, varname)
		} else if fastpathAvIndex(rtid) != -1 {
			g := x.newFastpathGenV(t)
			x.linef("z.F.%sX(%s%s, d)", g.MethodNamePfx("Dec", false), addrPfx, varname)
		} else {
			x.xtraSM(varname, t, false, isptr)
			// x.decListFallback(varname, rtid, false, t)
		}
	case reflect.Map:
		// if a known fastpath map, call dedicated function
		// else write encode function in-line.
		// - if elements are primitives or Selfers, call dedicated function on each member.
		// - else call Encoder.encode(XXX) on it.
		if fastpathAvIndex(rtid) != -1 {
			g := x.newFastpathGenV(t)
			x.linef("z.F.%sX(%s%s, d)", g.MethodNamePfx("Dec", false), addrPfx, varname)
		} else {
			x.xtraSM(varname, t, false, isptr)
		}
	case reflect.Struct:
		if inlist {
			// no need to create temp variable if isptr, or x.F or x[F]
			if isptr || strings.IndexByte(varname, '.') != -1 || strings.IndexByte(varname, '[') != -1 {
				x.decStruct(varname, rtid, t)
			} else {
				varname2 := genTempVarPfx + "j" + mi
				x.line(varname2 + " := &" + varname)
				x.decStruct(varname2, rtid, t)
			}
		} else {
			// delete(x.td, rtid)
			x.line("z.DecFallback(" + addrPfx + varname + ", false)")
		}
	default:
		if rtidAdded {
			delete(x.te, rtid)
		}
		x.line("z.DecFallback(" + addrPfx + varname + ", true)")
	}
}

func (x *genRunner) decTryAssignPrimitive(varname string, t reflect.Type, isptr bool) (done bool) {
	// This should only be used for exact primitives (ie un-named types).
	// Named types may be implementations of Selfer, Unmarshaler, etc.
	// They should be handled by dec(...)

	var ptr string
	if isptr {
		ptr = "*"
	}
	switch t.Kind() {
	case reflect.Int:
		x.linef("%s%s = (%s)(z.C.IntV(r.DecodeInt64(), codecSelferBitsize%s))", ptr, varname, x.genTypeName(t), x.xs)
	case reflect.Int8:
		x.linef("%s%s = (%s)(z.C.IntV(r.DecodeInt64(), 8))", ptr, varname, x.genTypeName(t))
	case reflect.Int16:
		x.linef("%s%s = (%s)(z.C.IntV(r.DecodeInt64(), 16))", ptr, varname, x.genTypeName(t))
	case reflect.Int32:
		x.linef("%s%s = (%s)(z.C.IntV(r.DecodeInt64(), 32))", ptr, varname, x.genTypeName(t))
	case reflect.Int64:
		x.linef("%s%s = (%s)(r.DecodeInt64())", ptr, varname, x.genTypeName(t))

	case reflect.Uint:
		x.linef("%s%s = (%s)(z.C.UintV(r.DecodeUint64(), codecSelferBitsize%s))", ptr, varname, x.genTypeName(t), x.xs)
	case reflect.Uint8:
		x.linef("%s%s = (%s)(z.C.UintV(r.DecodeUint64(), 8))", ptr, varname, x.genTypeName(t))
	case reflect.Uint16:
		x.linef("%s%s = (%s)(z.C.UintV(r.DecodeUint64(), 16))", ptr, varname, x.genTypeName(t))
	case reflect.Uint32:
		x.linef("%s%s = (%s)(z.C.UintV(r.DecodeUint64(), 32))", ptr, varname, x.genTypeName(t))
	case reflect.Uint64:
		x.linef("%s%s = (%s)(r.DecodeUint64())", ptr, varname, x.genTypeName(t))
	case reflect.Uintptr:
		x.linef("%s%s = (%s)(z.C.UintV(r.DecodeUint64(), codecSelferBitsize%s))", ptr, varname, x.genTypeName(t), x.xs)

	case reflect.Float32:
		x.linef("%s%s = (%s)(z.DecDecodeFloat32())", ptr, varname, x.genTypeName(t))
	case reflect.Float64:
		x.linef("%s%s = (%s)(r.DecodeFloat64())", ptr, varname, x.genTypeName(t))

	case reflect.Bool:
		x.linef("%s%s = (%s)(r.DecodeBool())", ptr, varname, x.genTypeName(t))
	case reflect.String:
		x.linef("%s%s = (%s)(string(r.DecodeStringAsBytes()))", ptr, varname, x.genTypeName(t))
	default:
		return false
	}
	return true
}

func (x *genRunner) decListFallback(varname string, rtid uintptr, t reflect.Type) {
	if t.AssignableTo(uint8SliceTyp) {
		x.line("*" + varname + " = r.DecodeBytes(*((*[]byte)(" + varname + ")), false)")
		return
	}
	if t.Kind() == reflect.Array && t.Elem().Kind() == reflect.Uint8 {
		x.linef("r.DecodeBytes( ((*[%d]byte)(%s))[:], true)", t.Len(), varname)
		return
	}
	type tstruc struct {
		TempVar   string
		Sfx       string
		Rand      string
		Varname   string
		CTyp      string
		Typ       string
		Immutable bool
		Size      int
	}
	telem := t.Elem()
	ts := tstruc{genTempVarPfx, x.xs, x.varsfx(), varname, x.genTypeName(t), x.genTypeName(telem), genIsImmutable(telem), int(telem.Size())}

	funcs := make(template.FuncMap)

	funcs["decLineVar"] = func(varname string) string {
		x.decVar(varname, "", telem, false, true)
		return ""
	}
	funcs["var"] = func(s string) string {
		return ts.TempVar + s + ts.Rand
	}
	funcs["xs"] = func() string {
		return ts.Sfx
	}
	funcs["zero"] = func() string {
		return x.genZeroValueR(telem)
	}
	funcs["isArray"] = func() bool {
		return t.Kind() == reflect.Array
	}
	funcs["isSlice"] = func() bool {
		return t.Kind() == reflect.Slice
	}
	funcs["isChan"] = func() bool {
		return t.Kind() == reflect.Chan
	}
	tm, err := template.New("").Funcs(funcs).Parse(genDecListTmpl)
	genCheckErr(err)
	genCheckErr(tm.Execute(x.w, &ts))
}

func (x *genRunner) decMapFallback(varname string, rtid uintptr, t reflect.Type) {
	type tstruc struct {
		TempVar string
		Sfx     string
		Rand    string
		Varname string
		KTyp    string
		Typ     string
		Size    int
	}
	telem := t.Elem()
	tkey := t.Key()
	ts := tstruc{
		genTempVarPfx, x.xs, x.varsfx(), varname, x.genTypeName(tkey),
		x.genTypeName(telem), int(telem.Size() + tkey.Size()),
	}

	funcs := make(template.FuncMap)
	funcs["decElemZero"] = func() string {
		return x.genZeroValueR(telem)
	}
	funcs["decElemKindImmutable"] = func() bool {
		return genIsImmutable(telem)
	}
	funcs["decElemKindPtr"] = func() bool {
		return telem.Kind() == reflect.Ptr
	}
	funcs["decElemKindIntf"] = func() bool {
		return telem.Kind() == reflect.Interface
	}
	funcs["decLineVarK"] = func(varname string) string {
		x.decVar(varname, "", tkey, false, true)
		return ""
	}
	funcs["decLineVar"] = func(varname, decodedNilVarname string) string {
		x.decVar(varname, decodedNilVarname, telem, false, true)
		return ""
	}
	funcs["var"] = func(s string) string {
		return ts.TempVar + s + ts.Rand
	}
	funcs["xs"] = func() string {
		return ts.Sfx
	}

	tm, err := template.New("").Funcs(funcs).Parse(genDecMapTmpl)
	genCheckErr(err)
	genCheckErr(tm.Execute(x.w, &ts))
}

func (x *genRunner) decStructMapSwitch(kName string, varname string, rtid uintptr, t reflect.Type) {
	ti := x.ti.get(rtid, t)
	tisfi := ti.sfiSrc // always use sequence from file. decStruct expects same thing.
	x.line("switch (" + kName + ") {")
	var newbuf, nilbuf genBuf
	for _, si := range tisfi {
		x.line("case \"" + si.encName + "\":")
		newbuf.reset()
		nilbuf.reset()
		varname3, t2 := x.decVarInitPtr(varname, "", t, si, &newbuf, &nilbuf)
		if len(newbuf.buf) > 0 {
			x.linef("if r.TryNil() { %s } else { %s", nilbuf.buf, newbuf.buf)
		}
		x.decVarMain(varname3, x.varsfx(), t2.Type, false)
		if len(newbuf.buf) > 0 {
			x.line("}")
		}
	}
	x.line("default:")
	// pass the slice here, so that the string will not escape, and maybe save allocation
	x.linef("z.DecStructFieldNotFound(-1, %s)", kName)
	x.linef("} // end switch %s", kName)
}

func (x *genRunner) decStructMap(varname, lenvarname string, rtid uintptr, t reflect.Type, style genStructMapStyle) {
	tpfx := genTempVarPfx
	ti := x.ti.get(rtid, t)
	i := x.varsfx()
	kName := tpfx + "s" + i

	switch style {
	case genStructMapStyleLenPrefix:
		x.linef("for %sj%s := 0; %sj%s < %s; %sj%s++ {", tpfx, i, tpfx, i, lenvarname, tpfx, i)
	case genStructMapStyleCheckBreak:
		x.linef("for %sj%s := 0; !z.DecCheckBreak(); %sj%s++ {", tpfx, i, tpfx, i)
	default: // 0, otherwise.
		x.linef("var %shl%s bool = %s >= 0", tpfx, i, lenvarname) // has length
		x.linef("for %sj%s := 0; ; %sj%s++ {", tpfx, i, tpfx, i)
		x.linef("if %shl%s { if %sj%s >= %s { break }", tpfx, i, tpfx, i, lenvarname)
		x.line("} else { if z.DecCheckBreak() { break }; }")
	}
	x.line("z.DecReadMapElemKey()")

	// emulate decstructfieldkey
	switch ti.keyType {
	case valueTypeInt:
		x.linef("%s := z.StringView(strconv.AppendInt(z.DecScratchArrayBuffer()[:0], r.DecodeInt64(), 10))", kName)
	case valueTypeUint:
		x.linef("%s := z.StringView(strconv.AppendUint(z.DecScratchArrayBuffer()[:0], r.DecodeUint64(), 10))", kName)
	case valueTypeFloat:
		x.linef("%s := z.StringView(strconv.AppendFloat(z.DecScratchArrayBuffer()[:0], r.DecodeFloat64(), 'f', -1, 64))", kName)
	default: // string
		x.linef("%s := z.StringView(r.DecodeStringAsBytes())", kName)
	}

	x.line("z.DecReadMapElemValue()")
	x.decStructMapSwitch(kName, varname, rtid, t)

	x.line("} // end for " + tpfx + "j" + i)
}

func (x *genRunner) decStructArray(varname, lenvarname, breakString string, rtid uintptr, t reflect.Type) {
	tpfx := genTempVarPfx
	i := x.varsfx()
	ti := x.ti.get(rtid, t)
	tisfi := ti.sfiSrc // always use sequence from file. decStruct expects same thing.
	x.linef("var %sj%s int", tpfx, i)
	x.linef("var %sb%s bool", tpfx, i)                        // break
	x.linef("var %shl%s bool = %s >= 0", tpfx, i, lenvarname) // has length
	if !genDecStructArrayInlineLoopCheck {
		x.linef("var %sfn%s = func() bool { ", tpfx, i)
		x.linef("%sj%s++; if %shl%s { %sb%s = %sj%s > %s } else { %sb%s = z.DecCheckBreak() };",
			tpfx, i, tpfx, i, tpfx, i,
			tpfx, i, lenvarname, tpfx, i)
		x.linef("if %sb%s { z.DecReadArrayEnd(); return true }; return false", tpfx, i)
		x.linef("} // end func %sfn%s", tpfx, i)
	}
	var newbuf, nilbuf genBuf
	for _, si := range tisfi {
		if genDecStructArrayInlineLoopCheck {
			x.linef("%sj%s++; if %shl%s { %sb%s = %sj%s > %s } else { %sb%s = z.DecCheckBreak() }",
				tpfx, i, tpfx, i, tpfx, i,
				tpfx, i, lenvarname, tpfx, i)
			x.linef("if %sb%s { z.DecReadArrayEnd(); %s }", tpfx, i, breakString)
		} else {
			x.linef("if %sfn%s() { %s }", tpfx, i, breakString)
		}
		x.line("z.DecReadArrayElem()")
		newbuf.reset()
		nilbuf.reset()
		varname3, t2 := x.decVarInitPtr(varname, "", t, si, &newbuf, &nilbuf)
		if len(newbuf.buf) > 0 {
			x.linef("if r.TryNil() { %s } else { %s", nilbuf.buf, newbuf.buf)
		}
		x.decVarMain(varname3, x.varsfx(), t2.Type, false)
		if len(newbuf.buf) > 0 {
			x.line("}")
		}
	}
	// read remaining values and throw away.
	x.line("for {")
	x.linef("%sj%s++; if %shl%s { %sb%s = %sj%s > %s } else { %sb%s = z.DecCheckBreak() }",
		tpfx, i, tpfx, i, tpfx, i,
		tpfx, i, lenvarname, tpfx, i)
	x.linef("if %sb%s { break }", tpfx, i)
	x.line("z.DecReadArrayElem()")
	x.linef(`z.DecStructFieldNotFound(%sj%s - 1, "")`, tpfx, i)
	x.line("}")
}

func (x *genRunner) decStruct(varname string, rtid uintptr, t reflect.Type) {
	// varname MUST be a ptr, or a struct field or a slice element.
	i := x.varsfx()
	x.linef("%sct%s := r.ContainerType()", genTempVarPfx, i)
	x.linef("if %sct%s == codecSelferValueTypeNil%s {", genTempVarPfx, i, x.xs)
	x.linef("*(%s) = %s{}", varname, x.genTypeName(t))
	x.linef("} else if %sct%s == codecSelferValueTypeMap%s {", genTempVarPfx, i, x.xs)
	x.line(genTempVarPfx + "l" + i + " := z.DecReadMapStart()")
	x.linef("if %sl%s == 0 {", genTempVarPfx, i)
	if genUseOneFunctionForDecStructMap {
		x.line("} else { ")
		x.linef("%s.codecDecodeSelfFromMap(%sl%s, d)", varname, genTempVarPfx, i)
	} else {
		x.line("} else if " + genTempVarPfx + "l" + i + " > 0 { ")
		x.line(varname + ".codecDecodeSelfFromMapLenPrefix(" + genTempVarPfx + "l" + i + ", d)")
		x.line("} else {")
		x.line(varname + ".codecDecodeSelfFromMapCheckBreak(" + genTempVarPfx + "l" + i + ", d)")
	}
	x.line("}")
	x.line("z.DecReadMapEnd()")

	// else if container is array
	x.linef("} else if %sct%s == codecSelferValueTypeArray%s {", genTempVarPfx, i, x.xs)
	x.line(genTempVarPfx + "l" + i + " := z.DecReadArrayStart()")
	x.linef("if %sl%s != 0 {", genTempVarPfx, i)
	x.linef("%s.codecDecodeSelfFromArray(%sl%s, d)", varname, genTempVarPfx, i)
	x.line("}")
	x.line("z.DecReadArrayEnd()")
	// else panic
	x.line("} else { ")
	x.line("panic(errCodecSelferOnlyMapOrArrayEncodeToStruct" + x.xs + ")")
	x.line("} ")
}

// --------

type fastpathGenV struct {
	// fastpathGenV is either a primitive (Primitive != "") or a map (MapKey != "") or a slice
	MapKey      string
	Elem        string
	Primitive   string
	Size        int
	NoCanonical bool
}

func (x *genRunner) newFastpathGenV(t reflect.Type) (v fastpathGenV) {
	v.NoCanonical = !genFastpathCanonical
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		te := t.Elem()
		v.Elem = x.genTypeName(te)
		v.Size = int(te.Size())
	case reflect.Map:
		te, tk := t.Elem(), t.Key()
		v.Elem = x.genTypeName(te)
		v.MapKey = x.genTypeName(tk)
		v.Size = int(te.Size() + tk.Size())
	default:
		halt.onerror(errGenUnexpectedTypeFastpath)
	}
	return
}

func (x *fastpathGenV) MethodNamePfx(prefix string, prim bool) string {
	var name []byte
	if prefix != "" {
		name = append(name, prefix...)
	}
	if prim {
		name = append(name, genTitleCaseName(x.Primitive)...)
	} else {
		if x.MapKey == "" {
			name = append(name, "Slice"...)
		} else {
			name = append(name, "Map"...)
			name = append(name, genTitleCaseName(x.MapKey)...)
		}
		name = append(name, genTitleCaseName(x.Elem)...)
	}
	return string(name)
}

// genImportPath returns import path of a non-predeclared named typed, or an empty string otherwise.
//
// This handles the misbehaviour that occurs when 1.5-style vendoring is enabled,
// where PkgPath returns the full path, including the vendoring pre-fix that should have been stripped.
// We strip it here.
func genImportPath(t reflect.Type) (s string) {
	s = t.PkgPath()
	if genCheckVendor {
		// HACK: always handle vendoring. It should be typically on in go 1.6, 1.7
		s = genStripVendor(s)
	}
	return
}

// A go identifier is (letter|_)[letter|number|_]*
func genGoIdentifier(s string, checkFirstChar bool) string {
	b := make([]byte, 0, len(s))
	t := make([]byte, 4)
	var n int
	for i, r := range s {
		if checkFirstChar && i == 0 && !unicode.IsLetter(r) {
			b = append(b, '_')
		}
		// r must be unicode_letter, unicode_digit or _
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n = utf8.EncodeRune(t, r)
			b = append(b, t[:n]...)
		} else {
			b = append(b, '_')
		}
	}
	return string(b)
}

func genNonPtr(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func genTitleCaseName(s string) string {
	switch s {
	case "interface{}", "interface {}":
		return "Intf"
	case "[]byte", "[]uint8", "bytes":
		return "Bytes"
	default:
		return strings.ToUpper(s[0:1]) + s[1:]
	}
}

func genMethodNameT(t reflect.Type, tRef reflect.Type) (n string) {
	var ptrPfx string
	for t.Kind() == reflect.Ptr {
		ptrPfx += "Ptrto"
		t = t.Elem()
	}
	tstr := t.String()
	if tn := t.Name(); tn != "" {
		if tRef != nil && genImportPath(t) == genImportPath(tRef) {
			return ptrPfx + tn
		} else {
			if genQNameRegex.MatchString(tstr) {
				return ptrPfx + strings.Replace(tstr, ".", "_", 1000)
			} else {
				return ptrPfx + genCustomTypeName(tstr)
			}
		}
	}
	switch t.Kind() {
	case reflect.Map:
		return ptrPfx + "Map" + genMethodNameT(t.Key(), tRef) + genMethodNameT(t.Elem(), tRef)
	case reflect.Slice:
		return ptrPfx + "Slice" + genMethodNameT(t.Elem(), tRef)
	case reflect.Array:
		return ptrPfx + "Array" + strconv.FormatInt(int64(t.Len()), 10) + genMethodNameT(t.Elem(), tRef)
	case reflect.Chan:
		var cx string
		switch t.ChanDir() {
		case reflect.SendDir:
			cx = "ChanSend"
		case reflect.RecvDir:
			cx = "ChanRecv"
		default:
			cx = "Chan"
		}
		return ptrPfx + cx + genMethodNameT(t.Elem(), tRef)
	default:
		if t == intfTyp {
			return ptrPfx + "Interface"
		} else {
			if tRef != nil && genImportPath(t) == genImportPath(tRef) {
				if t.Name() != "" {
					return ptrPfx + t.Name()
				} else {
					return ptrPfx + genCustomTypeName(tstr)
				}
			} else {
				// best way to get the package name inclusive
				// return ptrPfx + strings.Replace(tstr, ".", "_", 1000)
				// return ptrPfx + genBase64enc.EncodeToString([]byte(tstr))
				if t.Name() != "" && genQNameRegex.MatchString(tstr) {
					return ptrPfx + strings.Replace(tstr, ".", "_", 1000)
				} else {
					return ptrPfx + genCustomTypeName(tstr)
				}
			}
		}
	}
}

// genCustomNameForType base64encodes the t.String() value in such a way
// that it can be used within a function name.
func genCustomTypeName(tstr string) string {
	len2 := genBase64enc.EncodedLen(len(tstr))
	bufx := make([]byte, len2)
	genBase64enc.Encode(bufx, []byte(tstr))
	for i := len2 - 1; i >= 0; i-- {
		if bufx[i] == '=' {
			len2--
		} else {
			break
		}
	}
	return string(bufx[:len2])
}

func genIsImmutable(t reflect.Type) (v bool) {
	return isImmutableKind(t.Kind())
}

type genInternal struct {
	Version int
	Values  []fastpathGenV
}

func (x genInternal) FastpathLen() (l int) {
	for _, v := range x.Values {
		if v.Primitive == "" && !(v.MapKey == "" && v.Elem == "uint8") {
			l++
		}
	}
	return
}

func genInternalZeroValue(s string) string {
	switch s {
	case "interface{}", "interface {}":
		return "nil"
	case "[]byte", "[]uint8", "bytes":
		return "nil"
	case "bool":
		return "false"
	case "string":
		return `""`
	default:
		return "0"
	}
}

var genInternalNonZeroValueIdx [6]uint64
var genInternalNonZeroValueStrs = [...][6]string{
	{`"string-is-an-interface-1"`, "true", `"some-string-1"`, `[]byte("some-string-1")`, "11.1", "111"},
	{`"string-is-an-interface-2"`, "false", `"some-string-2"`, `[]byte("some-string-2")`, "22.2", "77"},
	{`"string-is-an-interface-3"`, "true", `"some-string-3"`, `[]byte("some-string-3")`, "33.3e3", "127"},
}

// Note: last numbers must be in range: 0-127 (as they may be put into a int8, uint8, etc)

func genInternalNonZeroValue(s string) string {
	var i int
	switch s {
	case "interface{}", "interface {}":
		i = 0
	case "bool":
		i = 1
	case "string":
		i = 2
	case "bytes", "[]byte", "[]uint8":
		i = 3
	case "float32", "float64", "float", "double":
		i = 4
	default:
		i = 5
	}
	genInternalNonZeroValueIdx[i]++
	idx := genInternalNonZeroValueIdx[i]
	slen := uint64(len(genInternalNonZeroValueStrs))
	return genInternalNonZeroValueStrs[idx%slen][i] // return string, to remove ambiguity
}

func genInternalEncCommandAsString(s string, vname string) string {
	switch s {
	case "uint64":
		return "e.e.EncodeUint(" + vname + ")"
	case "uint", "uint8", "uint16", "uint32":
		return "e.e.EncodeUint(uint64(" + vname + "))"
	case "int64":
		return "e.e.EncodeInt(" + vname + ")"
	case "int", "int8", "int16", "int32":
		return "e.e.EncodeInt(int64(" + vname + "))"
	case "[]byte", "[]uint8", "bytes":
		return "e.e.EncodeStringBytesRaw(" + vname + ")"
	case "string":
		return "e.e.EncodeString(" + vname + ")"
	case "float32":
		return "e.e.EncodeFloat32(" + vname + ")"
	case "float64":
		return "e.e.EncodeFloat64(" + vname + ")"
	case "bool":
		return "e.e.EncodeBool(" + vname + ")"
	// case "symbol":
	// 	return "e.e.EncodeSymbol(" + vname + ")"
	default:
		return "e.encode(" + vname + ")"
	}
}

func genInternalDecCommandAsString(s string) string {
	switch s {
	case "uint":
		return "uint(chkOvf.UintV(d.d.DecodeUint64(), uintBitsize))"
	case "uint8":
		return "uint8(chkOvf.UintV(d.d.DecodeUint64(), 8))"
	case "uint16":
		return "uint16(chkOvf.UintV(d.d.DecodeUint64(), 16))"
	case "uint32":
		return "uint32(chkOvf.UintV(d.d.DecodeUint64(), 32))"
	case "uint64":
		return "d.d.DecodeUint64()"
	case "uintptr":
		return "uintptr(chkOvf.UintV(d.d.DecodeUint64(), uintBitsize))"
	case "int":
		return "int(chkOvf.IntV(d.d.DecodeInt64(), intBitsize))"
	case "int8":
		return "int8(chkOvf.IntV(d.d.DecodeInt64(), 8))"
	case "int16":
		return "int16(chkOvf.IntV(d.d.DecodeInt64(), 16))"
	case "int32":
		return "int32(chkOvf.IntV(d.d.DecodeInt64(), 32))"
	case "int64":
		return "d.d.DecodeInt64()"

	case "string":
		return "string(d.d.DecodeStringAsBytes())"
	case "[]byte", "[]uint8", "bytes":
		return "d.d.DecodeBytes(nil, false)"
	case "float32":
		return "float32(d.decodeFloat32())"
	case "float64":
		return "d.d.DecodeFloat64()"
	case "bool":
		return "d.d.DecodeBool()"
	default:
		halt.onerror(errors.New("gen internal: unknown type for decode: " + s))
	}
	return ""
}

// func genInternalSortType(s string, elem bool) string {
// 	for _, v := range [...]string{
// 		"int",
// 		"uint",
// 		"float",
// 		"bool",
// 		"string",
// 		"bytes", "[]uint8", "[]byte",
// 	} {
// 		if v == "[]byte" || v == "[]uint8" {
// 			v = "bytes"
// 		}
// 		if strings.HasPrefix(s, v) {
// 			if v == "int" || v == "uint" || v == "float" {
// 				v += "64"
// 			}
// 			if elem {
// 				return v
// 			}
// 			return v + "Slice"
// 		}
// 	}
// 	halt.onerror(errors.New("sorttype: unexpected type: " + s))
// }

func genInternalSortType(s string, elem bool) string {
	if elem {
		return s
	}
	return s + "Slice"
}

// MARKER: keep in sync with codecgen/gen.go
func genStripVendor(s string) string {
	// HACK: Misbehaviour occurs in go 1.5. May have to re-visit this later.
	// if s contains /vendor/ OR startsWith vendor/, then return everything after it.
	const vendorStart = "vendor/"
	const vendorInline = "/vendor/"
	if i := strings.LastIndex(s, vendorInline); i >= 0 {
		s = s[i+len(vendorInline):]
	} else if strings.HasPrefix(s, vendorStart) {
		s = s[len(vendorStart):]
	}
	return s
}

// var genInternalMu sync.Mutex
var genInternalV = genInternal{Version: genVersion}
var genInternalTmplFuncs template.FuncMap
var genInternalOnce sync.Once

func genInternalInit() {
	wordSizeBytes := int(intBitsize) / 8

	typesizes := map[string]int{
		"interface{}": 2 * wordSizeBytes,
		"string":      2 * wordSizeBytes,
		"[]byte":      3 * wordSizeBytes,
		"uint":        1 * wordSizeBytes,
		"uint8":       1,
		"uint16":      2,
		"uint32":      4,
		"uint64":      8,
		"uintptr":     1 * wordSizeBytes,
		"int":         1 * wordSizeBytes,
		"int8":        1,
		"int16":       2,
		"int32":       4,
		"int64":       8,
		"float32":     4,
		"float64":     8,
		"bool":        1,
	}

	// keep as slice, so it is in specific iteration order.
	// Initial order was uint64, string, interface{}, int, int64, ...

	var types = [...]string{
		"interface{}",
		"string",
		"[]byte",
		"float32",
		"float64",
		"uint",
		"uint8",
		"uint16",
		"uint32",
		"uint64",
		"uintptr",
		"int",
		"int8",
		"int16",
		"int32",
		"int64",
		"bool",
	}

	var primitivetypes, slicetypes, mapkeytypes, mapvaltypes []string

	primitivetypes = types[:]
	slicetypes = types[:]
	mapkeytypes = types[:]
	mapvaltypes = types[:]

	if genFastpathTrimTypes {
		// Note: we only create fast-paths for commonly used types.
		// Consequently, things like int8, uint16, uint, etc are commented out.

		slicetypes = genInternalFastpathSliceTypes()
		mapkeytypes = genInternalFastpathMapKeyTypes()
		mapvaltypes = genInternalFastpathMapValueTypes()
	}

	// var mapkeytypes [len(&types) - 1]string // skip bool
	// copy(mapkeytypes[:], types[:])

	// var mb []byte
	// mb = append(mb, '|')
	// for _, s := range mapkeytypes {
	// 	mb = append(mb, s...)
	// 	mb = append(mb, '|')
	// }
	// var mapkeytypestr = string(mb)

	var gt = genInternal{Version: genVersion}

	// For each slice or map type, there must be a (symmetrical) Encode and Decode fast-path function

	for _, s := range primitivetypes {
		gt.Values = append(gt.Values,
			fastpathGenV{Primitive: s, Size: typesizes[s], NoCanonical: !genFastpathCanonical})
	}
	for _, s := range slicetypes {
		// if s != "uint8" { // do not generate fast path for slice of bytes. Treat specially already.
		// 	gt.Values = append(gt.Values, fastpathGenV{Elem: s, Size: typesizes[s]})
		// }
		gt.Values = append(gt.Values,
			fastpathGenV{Elem: s, Size: typesizes[s], NoCanonical: !genFastpathCanonical})
	}
	for _, s := range mapkeytypes {
		// if _, ok := typesizes[s]; !ok {
		// if strings.Contains(mapkeytypestr, "|"+s+"|") {
		// 	gt.Values = append(gt.Values, fastpathGenV{MapKey: s, Elem: s, Size: 2 * typesizes[s]})
		// }
		for _, ms := range mapvaltypes {
			gt.Values = append(gt.Values,
				fastpathGenV{MapKey: s, Elem: ms, Size: typesizes[s] + typesizes[ms], NoCanonical: !genFastpathCanonical})
		}
	}

	funcs := make(template.FuncMap)
	// funcs["haspfx"] = strings.HasPrefix
	funcs["encmd"] = genInternalEncCommandAsString
	funcs["decmd"] = genInternalDecCommandAsString
	funcs["zerocmd"] = genInternalZeroValue
	funcs["nonzerocmd"] = genInternalNonZeroValue
	funcs["hasprefix"] = strings.HasPrefix
	funcs["sorttype"] = genInternalSortType

	genInternalV = gt
	genInternalTmplFuncs = funcs
}

// genInternalGoFile is used to generate source files from templates.
func genInternalGoFile(r io.Reader, w io.Writer) (err error) {
	genInternalOnce.Do(genInternalInit)

	gt := genInternalV

	t := template.New("").Funcs(genInternalTmplFuncs)

	tmplstr, err := ioutil.ReadAll(r)
	if err != nil {
		return
	}

	if t, err = t.Parse(string(tmplstr)); err != nil {
		return
	}

	var out bytes.Buffer
	err = t.Execute(&out, gt)
	if err != nil {
		return
	}

	bout, err := format.Source(out.Bytes())
	if err != nil {
		w.Write(out.Bytes()) // write out if error, so we can still see.
		// w.Write(bout) // write out if error, as much as possible, so we can still see.
		return
	}
	w.Write(bout)
	return
}

func genInternalFastpathSliceTypes() []string {
	return []string{
		"interface{}",
		"string",
		"[]byte",
		// "float32",
		"float64",
		// "uint",
		// "uint8", // no need for fastpath of []uint8, as it is handled specially
		// "uint16",
		// "uint32",
		"uint64",
		// "uintptr",
		"int",
		// "int8",
		// "int16",
		"int32", // rune
		"int64",
		"bool",
	}
}

func genInternalFastpathMapKeyTypes() []string {
	return []string{
		// "interface{}",
		"string",
		// "[]byte",
		// "float32",
		// "float64",
		// "uint",
		"uint8",
		// "uint16",
		// "uint32",
		"uint64",
		// "uintptr",
		"int",
		// "int8",
		// "int16",
		// "int32",
		"int64",
		// "bool",
	}
}

func genInternalFastpathMapValueTypes() []string {
	return []string{
		"interface{}",
		"string",
		"[]byte",
		// "uint",
		"uint8",
		// "uint16",
		// "uint32",
		"uint64",
		// "uintptr",
		"int",
		//"int8",
		// "int16",
		// "int32", // rune (mostly used for unicode)
		"int64",
		// "float32",
		"float64",
		"bool",
	}
}

// sort-slice ...
// generates sort implementations for
// various slice types and combination slice+reflect.Value types.
//
// The combination slice+reflect.Value types are used
// during canonical encode, and the others are used during fast-path
// encoding of map keys.

// genInternalSortableTypes returns the types
// that are used for fast-path canonical's encoding of maps.
//
// For now, we only support the highest sizes for
// int64, uint64, float64, bool, string, bytes.
func genInternalSortableTypes() []string {
	return genInternalFastpathMapKeyTypes()
}

// genInternalSortablePlusTypes returns the types
// that are used for reflection-based canonical's encoding of maps.
//
// For now, we only support the highest sizes for
// int64, uint64, float64, bool, string, bytes.
func genInternalSortablePlusTypes() []string {
	return []string{
		"string",
		"float64",
		"uint64",
		// "uintptr",
		"int64",
		"bool",
		"time",
		"bytes",
	}
}

func genTypeForShortName(s string) string {
	switch s {
	case "time":
		return "time.Time"
	case "bytes":
		return "[]byte"
	}
	return s
}

func genArgs(args ...interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(args)/2)
	for i := 0; i < len(args); {
		m[args[i].(string)] = args[i+1]
		i += 2
	}
	return m
}

func genEndsWith(s0 string, sn ...string) bool {
	for _, s := range sn {
		if strings.HasSuffix(s0, s) {
			return true
		}
	}
	return false
}

func genCheckErr(err error) {
	halt.onerror(err)
}

func genRunSortTmpl2Go(fnameIn, fnameOut string) {
	var err error

	funcs := make(template.FuncMap)
	funcs["sortables"] = genInternalSortableTypes
	funcs["sortablesplus"] = genInternalSortablePlusTypes
	funcs["tshort"] = genTypeForShortName
	funcs["endswith"] = genEndsWith
	funcs["args"] = genArgs

	t := template.New("").Funcs(funcs)
	fin, err := os.Open(fnameIn)
	genCheckErr(err)
	defer fin.Close()
	fout, err := os.Create(fnameOut)
	genCheckErr(err)
	defer fout.Close()
	tmplstr, err := ioutil.ReadAll(fin)
	genCheckErr(err)
	t, err = t.Parse(string(tmplstr))
	genCheckErr(err)
	var out bytes.Buffer
	err = t.Execute(&out, 0)
	genCheckErr(err)
	bout, err := format.Source(out.Bytes())
	if err != nil {
		fout.Write(out.Bytes()) // write out if error, so we can still see.
	}
	genCheckErr(err)
	// write out if error, as much as possible, so we can still see.
	_, err = fout.Write(bout)
	genCheckErr(err)
}

func genRunTmpl2Go(fnameIn, fnameOut string) {
	// println("____ " + fnameIn + " --> " + fnameOut + " ______")
	fin, err := os.Open(fnameIn)
	genCheckErr(err)
	defer fin.Close()
	fout, err := os.Create(fnameOut)
	genCheckErr(err)
	defer fout.Close()
	err = genInternalGoFile(fin, fout)
	genCheckErr(err)
}

// --- some methods here for other types, which are only used in codecgen

// depth returns number of valid nodes in the hierachy
func (path *structFieldInfoPathNode) root() *structFieldInfoPathNode {
TOP:
	if path.parent != nil {
		path = path.parent
		goto TOP
	}
	return path
}

func (path *structFieldInfoPathNode) fullpath() (p []*structFieldInfoPathNode) {
	// this method is mostly called by a command-line tool - it's not optimized, and that's ok.
	// it shouldn't be used in typical runtime use - as it does unnecessary allocation.
	d := path.depth()
	p = make([]*structFieldInfoPathNode, d)
	for d--; d >= 0; d-- {
		p[d] = path
		path = path.parent
	}
	return
}
