// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gin

import (
	"github.com/gin-gonic/gin/binding"
	"log"
)

func (c *Context) GetCookie(name string) (string, error) {
	log.Println("GetCookie() method is deprecated. Use Cookie() instead.")
	return c.Cookie(name)
}

// BindWith binds the passed struct pointer using the specified binding engine.
// See the binding package.
func (c *Context) BindWith(obj interface{}, b binding.Binding) error {
	log.Println(`BindWith(\"interface{}, binding.Binding\") error is going to
	be deprecated, please check issue #662 and either use MustBindWith() if you
	want HTTP 400 to be automatically returned if any error occur, of use
	ShouldBindWith() if you need to manage the error.`)
	return c.MustBindWith(obj, b)
}
