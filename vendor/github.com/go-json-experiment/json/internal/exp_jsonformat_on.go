// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (!goexperiment.jsonv2 || !go1.25) && goexperiment.jsonformat

package internal

const ExpJSONFormat = true
