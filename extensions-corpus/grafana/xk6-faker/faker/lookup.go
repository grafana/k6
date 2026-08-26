package faker

import (
	"sort"
	"sync"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/iancoleman/strcase"
)

// GetFuncLookups returns fake functions lookup table.
func GetFuncLookups() map[string]*gofakeit.Info {
	requireFuncLookups()

	return _funcLookups
}

func getCategoryNames() []string {
	requireFuncLookups()

	return _categoryNames
}

// GetCategoryFuncs returns fake functions by category.
func GetCategoryFuncs() map[string]map[string]*gofakeit.Info {
	requireFuncLookups()

	return _categoryFuncs
}

func lookupCategory(name string) (map[string]*gofakeit.Info, bool) {
	requireFuncLookups()

	funcs, ok := _categoryFuncs[name]

	return funcs, ok
}

func lookupFunc(name string) (*gofakeit.Info, bool) {
	requireFuncLookups()

	fun, ok := _funcLookups[name]

	return fun, ok
}

//nolint:gochecknoglobals
var (
	convertLookupsOnce sync.Once

	_funcLookups   map[string]*gofakeit.Info
	_categoryNames []string
	_categoryFuncs map[string]map[string]*gofakeit.Info
)

func requireFuncLookups() {
	convertLookupsOnce.Do(convertFuncLookups)
}

//nolint:gochecknoglobals
var (
	funcToSkip = map[string]struct{}{
		"template":    {},
		"generate":    {},
		"weighted":    {},
		"imagejpeg":   {},
		"imagepng":    {},
		"imagesvg":    {},
		"svg":         {},
		"sql":         {},
		"fixed_width": {},
		"map":         {},
		"regex":       {},
		"json":        {},
		"xml":         {},
		"csv":         {},
		"email_text":  {},
		"markdown":    {},
		"vowel":       {},
		"flipacoin":   {},
	}

	addPrefix = map[string]string{
		"booktitle":  "Book",
		"bookauthor": "Book",
		"bookgenre":  "Book",
		"moviegenre": "Movie",
	}

	funcRename = map[string]string{
		"gRpcError":     "gRPCError",
		"creditCardCvv": "creditCardCVV",
	}

	categoryRename = map[string]string{
		"auth":   "internet",
		"image":  "internet",
		"html":   "internet",
		"school": "person",
		"string": "strings",
		"number": "numbers",
	}

	categoryByFunc = map[string]string{
		"uuid":      "string",
		"flipACoin": "string",
		"boolean":   "number",
	}
)

func fixLookup(name string, info *gofakeit.Info) string {
	if prefix, need := addPrefix[name]; need {
		info.Display = prefix + " " + info.Display
	}

	key := strcase.ToLowerCamel(info.Display)

	if fixed, need := funcRename[key]; need {
		key = fixed
	}

	if fixed, need := categoryByFunc[key]; need {
		info.Category = fixed
	}

	if fixed, need := categoryRename[info.Category]; need {
		info.Category = fixed
	}

	return key
}

func convertFuncLookups() {
	_funcLookups = make(map[string]*gofakeit.Info)
	_categoryFuncs = make(map[string]map[string]*gofakeit.Info)
	zen := make(map[string]*gofakeit.Info)

	for key, info := range gofakeit.FuncLookups {
		if _, skip := funcToSkip[key]; skip {
			continue
		}

		key = fixLookup(key, &info)
		_funcLookups[key] = &info

		category, ok := _categoryFuncs[info.Category]
		if !ok {
			category = make(map[string]*gofakeit.Info)
			_categoryFuncs[info.Category] = category
		}

		category[key] = &info
		zen[key] = &info
	}

	_categoryFuncs["zen"] = zen

	_categoryNames = make([]string, 0, len(_categoryFuncs))

	for name := range _categoryFuncs {
		_categoryNames = append(_categoryNames, name)
	}

	sort.Strings(_categoryNames)
}
