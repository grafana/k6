// +build !appengine

package fasttemplate

import (
	"reflect"
	"unsafe"
)

func unsafeBytes2String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func unsafeString2Bytes(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
}
