//go:build codegen

package main

import (
	"fmt"
	"io"
)

const itProlog = `let mod = require("k6/x/faker");
let faker = new mod.Faker(11);

function exists(v, msg) {
  if (typeof(v) == "undefined" || v == null) throw Error(msg);
}

`

func itGen(out io.Writer) error {
	fmt.Fprint(out, itProlog)

	catfuncs := getCategoryFuncs()

	for _, cname := range keys(catfuncs) {
		funcs := catfuncs[cname]

		for _, fun := range keys(funcs) {
			if fun == "creditCardNumber" { // it is not worth generating due to complicated parameter conditions
				continue
			}

			info := funcs[fun]

			params, err := genParams(info)
			if err != nil {
				return err
			}

			callparams := params
			if len(callparams) != 0 {
				callparams = "," + callparams
			}

			fmt.Fprintf(out, "exists(faker.%s.%s(%s), '%s.%s(%s)');\n", cname, fun, params, cname, fun, params)
			if cname == "zen" {
				fmt.Fprintf(out, "exists(faker.call(\"%s\"%s), 'call(\"%s\"%s)');\n",
					fun, callparams, fun, callparams)
			}
		}
	}

	return nil
}
