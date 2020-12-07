// +build !safe
// +build !appengine
// +build go1.7
// +build !gc

// Copyright (c) 2012-2020 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import "reflect"

var unsafeZeroArr [1024]byte

func rvType(rv reflect.Value) reflect.Type {
	return rv.Type()
}
