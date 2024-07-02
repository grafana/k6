// Package csv provides a CSV parser for k6.
package csv

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/js/promises"

	"go.k6.io/k6/js/modules/k6/experimental/fs"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

// TODO: Should we have an option to cycle through the file (restart from start when EOF is reached, or done is true)?
// TODO: Should we have an option to skip empty lines (lines with no fields?)
// TODO: Should we have an option to skip lines based on a specific predicate func(linenum, fields): bool ?:w

type (
	// RootModule is the global module instance that will create instances of our
	// module for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the fs module for a single VU.
	ModuleInstance struct {
		vu modules.VU
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new [RootModule] instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface and returns a new
// instance of our module for the given VU.
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{vu: vu}
}

// Exports implements the modules.Module interface and returns the exports of
// our module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"Parser": mi.NewParser,
		},
	}
}

// Parser is a CSV parser.
type Parser struct {
	CurrentLine atomic.Int64

	// reader is the CSV reader that enables to read records from the provided
	// input file.
	reader *csv.Reader

	// options holds the parser's as provided by the user.
	options parserOptions

	// vu is the VU instance that owns this module instance.
	vu modules.VU
}

// NewParser creates a new CSV parser instance.
func (mi *ModuleInstance) NewParser(call goja.ConstructorCall) *goja.Object {
	rt := mi.vu.Runtime()

	if len(call.Arguments) < 1 || goja.IsUndefined(call.Argument(0)) {
		common.Throw(rt, fmt.Errorf("parser constructor takes at least one non-nil source argument"))
	}

	// 1. Make sure the Goja object is a fs.File (goja operation)
	var file fs.File
	if err := mi.vu.Runtime().ExportTo(call.Argument(0), &file); err != nil {
		common.Throw(mi.vu.Runtime(), fmt.Errorf("first arg doesn't appear to be a *file.File instance"))
	}

	var options parserOptions
	if len(call.Arguments) == 2 && !goja.IsUndefined(call.Argument(1)) {
		var err error
		options, err = newParserOptionsFrom(call.Argument(1).ToObject(rt))
		if err != nil {
			common.Throw(rt, fmt.Errorf("encountered an error while interpreting Parser options; reason: %w", err))
		}
	}

	// Instantiate and configure csv reader
	r := csv.NewReader(file.ReadSeekStater)
	r.ReuseRecord = true        // evaluate if this is needed, and if it leads to unforeseen issues
	r.Comma = options.Delimiter // default delimiter, should be modifiable by the user

	var (
		fromLineSet      = options.FromLine.Valid
		skipFirstLineSet = options.SkipFirstLine
	)

	// If the user wants to skip the first line, we consume and discard it.
	if skipFirstLineSet && (!fromLineSet || options.FromLine.Int64 == 0) {
		_, err := r.Read()
		if err != nil {
			common.Throw(rt, fmt.Errorf("failed to skip the first line; reason: %w", err))
		}
	}

	// If the user wants to start reading from a specific line, we read and discard
	// lines until we reach the desired line.
	if fromLineSet && options.FromLine.Int64 > 0 {
		for i := int64(0); i < options.FromLine.Int64; i++ {
			_, err := r.Read()
			if err != nil {
				common.Throw(rt, fmt.Errorf("failed to iterate to fromLine; reason: %w", err))
			}
		}
	}

	// Create a new Parser instance
	parser := Parser{
		reader:  r,
		options: options,
		vu:      mi.vu,
	}

	return rt.ToValue(&parser).ToObject(rt)
}

// Next returns the next row in the CSV file.
func (p *Parser) Next() *goja.Promise {
	promise, resolve, reject := promises.New(p.vu)

	go func() {
		var records []string
		var done bool
		var err error

		if p.options.ToLine.Valid && p.CurrentLine.Load() >= p.options.ToLine.Int64 {
			done = true
			resolve(parseResult{done, records})
			return
		}

		records, err = p.reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				done = true
			} else {
				reject(err)
				return
			}
		}

		p.CurrentLine.Add(1)

		resolve(parseResult{done, records})
	}()

	return promise
}

// parseResult holds the result of a CSV parser's parsing operation such
// as when calling the [Next].
type parseResult struct {
	// Done indicates whether the parser has finished reading the file.
	Done bool `js:"done"`

	// Value holds the line's records value.
	Value []string `js:"value"`
}

// parserOptions holds the options for the CSV parser.
type parserOptions struct {
	// Delimiter is the character that separates the fields in the CSV.
	Delimiter rune `js:"delimiter"`

	// SkipFirstLine indicates whether the first line should be skipped.
	SkipFirstLine bool `js:"skipFirstLine"`

	// FromLine indicates the line from which to start reading the CSV file.
	FromLine null.Int `js:"fromLine"`

	// ToLine indicates the line at which to stop reading the CSV file.
	ToLine null.Int `js:"toLine"`
}

// newParserOptions creates a new parserOptions instance from the given Goja object.
func newParserOptionsFrom(obj *goja.Object) (parserOptions, error) {
	// Initialize default options
	options := parserOptions{
		Delimiter:     ',',
		SkipFirstLine: false,
	}

	if obj == nil {
		return options, nil
	}

	if v := obj.Get("delimiter"); v != nil {
		delimiter := v.String()

		// A delimiter is gonna be treated as a rune in the Go code, so we need to make sure it's a single character.
		if len(delimiter) > 1 {
			return options, fmt.Errorf("delimiter must be a single character")
		}

		options.Delimiter = rune(delimiter[0])
	}

	if v := obj.Get("skipFirstLine"); v != nil {
		options.SkipFirstLine = v.ToBoolean()
	}

	if v := obj.Get("fromLine"); v != nil {
		options.FromLine = null.IntFrom(v.ToInteger())
	}

	if v := obj.Get("toLine"); v != nil {
		options.ToLine = null.IntFrom(v.ToInteger())
	}

	if options.FromLine.Valid && options.ToLine.Valid && options.FromLine.Int64 > options.ToLine.Int64 {
		return options, fmt.Errorf("fromLine must be less than or equal to toLine")
	}

	return options, nil
}
