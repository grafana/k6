// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonopts

import (
	"github.com/go-json-experiment/json/internal"
	"github.com/go-json-experiment/json/internal/jsonflags"
)

// Options is the common options type shared across json packages.
type Options interface {
	// JSONOptions is exported so related json packages can implement Options.
	JSONOptions(internal.NotForPublicUse)
}

// Struct is the combination of all options in struct form.
// This is efficient to pass down the call stack and to query.
type Struct struct {
	Flags jsonflags.Flags

	CoderValues
	ArshalValues
}

type CoderValues struct {
	Indent       string // jsonflags.Indent
	IndentPrefix string // jsonflags.IndentPrefix
	ByteLimit    int64  // jsonflags.ByteLimit
	DepthLimit   int    // jsonflags.DepthLimit
}

type ArshalValues struct {
	// The Marshalers and Unmarshalers fields use the any type to avoid a
	// concrete dependency on *json.Marshalers and *json.Unmarshalers,
	// which would in turn create a dependency on the "reflect" package.

	Marshalers   any // jsonflags.Marshalers
	Unmarshalers any // jsonflags.Unmarshalers

	Format      string
	FormatDepth int
}

// DefaultOptionsV2 is the set of all options that define default v2 behavior.
var DefaultOptionsV2 = Struct{
	Flags: jsonflags.Flags{
		Presence: uint64(jsonflags.AllFlags & ^jsonflags.WhitespaceFlags),
		Values:   uint64(0),
	},
}

// DefaultOptionsV1 is the set of all options that define default v1 behavior.
var DefaultOptionsV1 = Struct{
	Flags: jsonflags.Flags{
		Presence: uint64(jsonflags.AllFlags & ^jsonflags.WhitespaceFlags),
		Values:   uint64(jsonflags.DefaultV1Flags),
	},
}

// CopyCoderOptions copies coder-specific options from src to dst.
// This is used by json.MarshalEncode and json.UnmarshalDecode since those
// functions ignore any coder-specific options and uses the options from the
// Encoder or Decoder that is passed in.
func (dst *Struct) CopyCoderOptions(src *Struct) {
	srcFlags := src.Flags
	srcFlags.Clear(^jsonflags.AllCoderFlags)
	dst.Flags.Join(srcFlags)
	dst.CoderValues = src.CoderValues
}

func (*Struct) JSONOptions(internal.NotForPublicUse) {}

// GetUnknownOption is injected by the "json" package to handle Options
// declared in that package so that "jsonopts" can handle them.
var GetUnknownOption = func(*Struct, Options) (any, bool) { panic("unknown option") }

func GetOption[T any](opts Options, setter func(T) Options) (T, bool) {
	// Collapse the options to *Struct to simplify lookup.
	structOpts, ok := opts.(*Struct)
	if !ok {
		var structOpts2 Struct
		structOpts2.Join(opts)
		structOpts = &structOpts2
	}

	// Lookup the option based on the return value of the setter.
	var zero T
	switch opt := setter(zero).(type) {
	case jsonflags.Bools:
		v := structOpts.Flags.Get(opt)
		ok := structOpts.Flags.Has(opt)
		return any(v).(T), ok
	case Indent:
		if !structOpts.Flags.Has(jsonflags.Indent) {
			return zero, false
		}
		return any(structOpts.Indent).(T), true
	case IndentPrefix:
		if !structOpts.Flags.Has(jsonflags.IndentPrefix) {
			return zero, false
		}
		return any(structOpts.IndentPrefix).(T), true
	case ByteLimit:
		if !structOpts.Flags.Has(jsonflags.ByteLimit) {
			return zero, false
		}
		return any(structOpts.ByteLimit).(T), true
	case DepthLimit:
		if !structOpts.Flags.Has(jsonflags.DepthLimit) {
			return zero, false
		}
		return any(structOpts.DepthLimit).(T), true
	default:
		v, ok := GetUnknownOption(structOpts, opt)
		return v.(T), ok
	}
}

// JoinUnknownOption is injected by the "json" package to handle Options
// declared in that package so that "jsonopts" can handle them.
var JoinUnknownOption = func(*Struct, Options) { panic("unknown option") }

func (dst *Struct) Join(srcs ...Options) {
	for _, src := range srcs {
		switch src := src.(type) {
		case nil:
			continue
		case jsonflags.Bools:
			dst.Flags.Set(src)
		case Indent:
			dst.Flags.Set(jsonflags.Multiline | jsonflags.Indent | 1)
			dst.Indent = string(src)
		case IndentPrefix:
			dst.Flags.Set(jsonflags.Multiline | jsonflags.IndentPrefix | 1)
			dst.IndentPrefix = string(src)
		case ByteLimit:
			dst.Flags.Set(jsonflags.ByteLimit | 1)
			dst.ByteLimit = int64(src)
		case DepthLimit:
			dst.Flags.Set(jsonflags.DepthLimit | 1)
			dst.DepthLimit = int(src)
		case *Struct:
			dst.Flags.Join(src.Flags)
			if src.Flags.Has(jsonflags.NonBooleanFlags) {
				if src.Flags.Has(jsonflags.Indent) {
					dst.Indent = src.Indent
				}
				if src.Flags.Has(jsonflags.IndentPrefix) {
					dst.IndentPrefix = src.IndentPrefix
				}
				if src.Flags.Has(jsonflags.ByteLimit) {
					dst.ByteLimit = src.ByteLimit
				}
				if src.Flags.Has(jsonflags.DepthLimit) {
					dst.DepthLimit = src.DepthLimit
				}
				if src.Flags.Has(jsonflags.Marshalers) {
					dst.Marshalers = src.Marshalers
				}
				if src.Flags.Has(jsonflags.Unmarshalers) {
					dst.Unmarshalers = src.Unmarshalers
				}
			}
			if src.Format != "" {
				dst.Format = src.Format
				dst.FormatDepth = src.FormatDepth
			}
		default:
			JoinUnknownOption(dst, src)
		}
	}
}

type (
	Indent       string // jsontext.WithIndent
	IndentPrefix string // jsontext.WithIndentPrefix
	ByteLimit    int64  // jsontext.WithByteLimit
	DepthLimit   int    // jsontext.WithDepthLimit
	// type for jsonflags.Marshalers declared in "json" package
	// type for jsonflags.Unmarshalers declared in "json" package
)

func (Indent) JSONOptions(internal.NotForPublicUse)       {}
func (IndentPrefix) JSONOptions(internal.NotForPublicUse) {}
func (ByteLimit) JSONOptions(internal.NotForPublicUse)    {}
func (DepthLimit) JSONOptions(internal.NotForPublicUse)   {}
