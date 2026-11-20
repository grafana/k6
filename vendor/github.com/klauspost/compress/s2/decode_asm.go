// Copyright 2016 The Snappy-Go Authors. All rights reserved.
// Copyright (c) 2019 Klaus Post. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (amd64 || arm64) && gc && !noasm

package s2

// decode has the same semantics as in decode_other.go.
//
//go:noescape
func s2Decode(dst, src []byte) int
