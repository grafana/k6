//go:build codegen

package main

import (
	"strings"

	"github.com/grafana/xk6-faker/faker"

	"github.com/brianvoe/gofakeit/v6"
)

func typemap(src string) string {
	var array bool
	if array = strings.HasPrefix(src, "[]"); array {
		src = src[2:]
	}

	switch src {
	case "string":
	case "bool":
		src = "boolean"
	case "float", "float32", "float64":
		fallthrough
	case "byte", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		src = "number"
	case "map[string]any":
		src = "Record<string,unknown>"
	case "map[string]string":
		src = "Record<string,string>"
	case "any":
		src = "unknown"
	case "map[string][]string":
		src = "Record<string, Array<string>>"
	default:
		return ""
	}

	if array {
		src += "[]"
	}

	return src
}

func convertLookup(src *gofakeit.Info) (*gofakeit.Info, bool) {
	output := typemap(src.Output)
	if len(output) == 0 {
		return src, false
	}

	info := *src

	info.Output = output

	if len(src.Params) == 0 {
		return &info, true
	}

	info.Params = make([]gofakeit.Param, len(src.Params))

	for idx, from := range src.Params {
		if from.Field == "enddate" {
			from.Default = "2024-03-13" // v0.3.0 release date
		}

		param := from
		param.Type = typemap(param.Type)

		info.Params[idx] = param
	}

	info.Category = convertCategory(src.Category)

	return &info, true
}

func getFuncLookups() map[string]*gofakeit.Info {
	all := make(map[string]*gofakeit.Info)

	for key, value := range faker.GetFuncLookups() {
		if info, ok := convertLookup(value); ok {
			all[key] = info
		}
	}

	return all
}

func getCategoryFuncs() map[string]map[string]*gofakeit.Info {
	all := make(map[string]map[string]*gofakeit.Info)

	for cname, funcs := range faker.GetCategoryFuncs() {
		category := make(map[string]*gofakeit.Info, len(funcs))

		for fun, info := range funcs {
			if converted, ok := convertLookup(info); ok {
				category[fun] = converted
			}
		}

		all[convertCategory(cname)] = category
	}

	return all
}

func convertCategory(name string) string {
	switch name {
	case "string":
		return "strings"
	case "number":
		return "numbers"
	default:
		return name
	}
}
