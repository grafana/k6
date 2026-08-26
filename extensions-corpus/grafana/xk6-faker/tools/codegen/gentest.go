//go:build codegen

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
)

const testProlog = `import { check, group } from "k6";
import { Faker } from "k6/x/faker";

export const options = {
  "thresholds": {
    "checks": ["rate == 1.0"]
  }
};

export default function () {
  let faker = new Faker(11);
  let checker = (v) => typeof(v) != "undefined";
`

func genParams(info *gofakeit.Info) (string, error) {
	faker := gofakeit.New(11)

	if len(info.Params) == 0 {
		return "", nil
	}

	var buff strings.Builder

	min := 2

	for idx, param := range info.Params {
		if idx > 0 {
			buff.WriteRune(',')
		}

		var val any

		switch param.Type {
		case "number":
			val = faker.Number(min, min+3)
			min += 3
		case "boolean":
			val = faker.Bool()
		case "string":
			val = faker.Word()
		case "string[]":
			str := faker.Sentence(10)
			str = strings.ToLower(str[:len(str)-1])
			val = strings.Split(str, " ")
		case "number[]":
			val = []any{faker.Number(5, 20), faker.Number(2, 10), faker.Number(3, 15)}
		default:
			val = nil
		}

		if param.Type == "string" && len(param.Default) != 0 {
			val = param.Default
		}

		if param.Type == "number" && len(param.Default) != 0 {
			if v, e := strconv.Atoi(param.Default); e == nil {
				val = v
			}
		}

		b, err := json.Marshal(val)
		if err != nil {
			return "", err
		}

		buff.Write(b)
	}

	return buff.String(), nil
}

func keys[VT any](dict map[string]VT) []string {
	keys := make([]string, 0, len(dict))
	for key := range dict {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func goTest(out io.Writer) error {
	fmt.Fprint(out, testProlog)

	catfuncs := getCategoryFuncs()

	for _, cname := range keys(catfuncs) {
		funcs := catfuncs[cname]
		fmt.Fprintf(out, "  group('%s', ()=> {\n", cname)
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

			fmt.Fprintf(out, "    check(faker.%s.%s(%s), { '%s.%s(%s)': checker });\n", cname, fun, params, cname, fun, params)
			if cname == "zen" {
				fmt.Fprintf(out, "    check(faker.call(\"%s\"%s), { 'call(\"%s\"%s)': checker });\n",
					fun, callparams, fun, callparams)
			}
		}
		fmt.Fprintf(out, "  });\n")
	}

	fmt.Fprintln(out, "};")

	return nil
}
