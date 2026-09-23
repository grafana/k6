// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import "sync/atomic"

var globalEnableFormatTag atomic.Bool

// ExperimentalGlobalSupportFormatTag globally enables
// [ExperimentalSupportFormatTag] for all calls to
// [Marshal], [MarshalWrite], [MarshalEncode],
// [Unmarshal], [UnmarshalRead], or [UnmarshalDecode].
//
// WARNING: This is an experimental feature and will be removed in the future
// as either a failed experiment or be formally included in "github.com/go-json-experiment/json"
// in some semantically similar form, in which case, users of this option
// must migrate to the officially supported feature.
func ExperimentalGlobalSupportFormatTag(v bool) {
	globalEnableFormatTag.Store(v)
}

func mayAppendSupportFormatTag(opts []Options) []Options {
	if globalEnableFormatTag.Load() {
		var optsArr [4]Options
		opts = append(append(optsArr[:0], opts...), &enableFormatTag)
	}
	return opts
}
