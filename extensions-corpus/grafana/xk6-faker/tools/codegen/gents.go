//go:build codegen

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/grafana/sobek"
	"github.com/grafana/xk6-faker/faker"
	"github.com/grafana/xk6-faker/module"
	"github.com/iancoleman/strcase"
)

//go:embed prolog.d.ts
var tsProlog []byte

const tsEpilog = `
/** Default Faker instance. */
declare const faker: Faker;

/** Default Faker instance */
export default faker;

`

func tsGen(out io.Writer) error {
	tsProlog[bytes.LastIndexByte(tsProlog, '}')] = '\n'
	fmt.Fprint(out, string(tsProlog))

	for idx, cname := range keys(faker.GetCategoryFuncs()) {
		if idx != 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "  /**\n   * %s\n   */\n", catdesc[cname])
		fmt.Fprintf(out, "  readonly %s: %s;\n", cname, strcase.ToCamel(cname))
	}

	fmt.Fprintln(out, "}")
	fmt.Fprint(out, tsEpilog)

	categories := getCategoryFuncs()

	for idx, cname := range keys(categories) {
		if idx != 0 {
			fmt.Fprintln(out)
		}

		fmt.Fprintf(out, "/**\n * %s\n */\n", catdesc[cname])
		fmt.Fprintf(out, "export declare interface %s {\n", strcase.ToCamel(cname))

		funcs := categories[cname]

		for fidx, fname := range keys(funcs) {
			if fidx != 0 {
				fmt.Fprintln(out)
			}

			info := funcs[fname]

			fmt.Fprintf(out, "  /**\n   * %s.\n", info.Description)

			for _, param := range info.Params {
				fmt.Fprintf(out, "   * @param %s - %s\n", param.Field, param.Display)
			}

			fmt.Fprintf(out, "   * @returns a random %s\n", strings.ToLower(info.Display))
			fmt.Fprintf(out, "   * @example\n")
			fmt.Fprintf(out, "   * ```ts\n")

			example, output, err := buildExample(fname, cname, info)
			if err != nil {
				return err
			}

			for _, line := range strings.Split(example, "\n") {
				fmt.Fprintf(out, "   *%s\n", line)
			}

			fmt.Fprintf(out, "   *```\n")

			fmt.Fprintf(out, "   * **Output** (formatted as JSON value)\n")
			fmt.Fprintf(out, "   *```json\n")
			fmt.Fprintf(out, "   * %s\n", output)
			fmt.Fprintf(out, "   * ```\n")
			fmt.Fprintf(out, "   */\n")
			fmt.Fprintf(out, "  %s(%s): %s;\n", fname, buildParamList(info), info.Output)
		}

		fmt.Fprintln(out, "}")
	}

	return nil
}

func buildParamList(info *gofakeit.Info) string {
	out := new(bytes.Buffer)

	for idx, param := range info.Params {
		if idx != 0 {
			fmt.Fprint(out, ", ")
		}

		fmt.Fprintf(out, "%s: %s", param.Field, param.Type)
	}

	return out.String()
}

func buildExample(name string, category string, info *gofakeit.Info) (string, string, error) {
	params, err := genParams(info)
	if err != nil {
		return "", "", err
	}

	call := fmt.Sprintf("faker.%s.%s(%s)", category, name, params)

	runtime := sobek.New()
	err = runtime.Set("Faker", faker.Constructor)
	if err != nil {
		return "", "", err
	}

	value, err := runtime.RunString(fmt.Sprintf("let faker=new Faker(11); %s", call))
	if err != nil {
		return "", "", err
	}

	var output string

	if obj := value.ToObject(runtime); obj != nil {
		b, err := obj.MarshalJSON()
		if err != nil {
			return "", "", err
		}
		output = string(b)
	} else {
		output = value.ToString().String()
	}

	out := new(bytes.Buffer)

	fmt.Fprintf(out, `import { Faker } from "%s"`, module.ImportPath)
	fmt.Fprintf(out, `

let faker = new Faker(11)

export default function () {
  console.log(%s)
}
`, call)

	return out.String(), output, nil
}

var catdesc = map[string]string{ //nolint:gochecknoglobals
	"address":   "Generator to generate addresses and locations.",
	"animal":    "Generator to generate animals.",
	"app":       "Generator to generate application related entries.",
	"beer":      "Generator to generate beer related entries.",
	"book":      "Generator to generate book related entries.",
	"car":       "Generator to generate car related entries.",
	"celebrity": "Generator to generate celebrities.",
	"color":     "Generator to generate colors.",
	"company":   "Generator to generate company related entries.",
	"emoji":     "Generator to generate emoji related entries.",
	"error":     "Generator to generate various error codes and messages.",
	"file":      "Generator to generate file related entries.",
	"finance":   "Generator to generate finance related entries.",
	"food":      "Generator to generate food related entries.",
	"game":      "Generator to generate game related entries.",
	"hacker":    "Generator to generate hacker/IT words and phrases.",
	"hipster":   "Generator to generate hipster words, phrases and paragraphs.",
	"internet":  "Generator to generate internet related entries.",
	"language":  "Generator to generate language related entries.",
	"minecraft": "Generator to generate minecraft related entries.",
	"movie":     "Generator to generate movie related entries.",
	"numbers":   "Generator to generate numbers.",
	"payment":   "Generator to generate payment related entries.",
	"person":    "Generator to generate people's personal information.",
	"product":   "Generator to generate product related entries.",
	"strings":   "Generator to generate strings.",
	"time":      "Generator to generate time and date.",
	"word":      "Generator to generate words and sentences.",
	"zen":       "Generator with all generator functions for convenient use.",
}
