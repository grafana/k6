//+build go1.8

package negroni

import (
	"fmt"
	"net/http"
)

func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := rw.ResponseWriter.(http.Pusher)
	if ok {
		return pusher.Push(target, opts)
	}
	return fmt.Errorf("the ResponseWriter doesn't support the Pusher interface")
}
