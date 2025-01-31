// Package csv provides a CSV parser for k6.
package csv

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"time"

	"go.k6.io/k6/internal/js/modules/k6/data"

	"github.com/grafana/sobek"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/js/promises"

	"go.k6.io/k6/internal/js/modules/k6/experimental/fs"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create instances of our
	// module for each VU.
	RootModule struct {
		dataModuleInstance *data.Data
	}

	// ModuleInstance represents an instance of the fs module for a single VU.
	ModuleInstance struct {
		vu modules.VU

		*RootModule
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
	if rm.dataModuleInstance == nil {
		var ok bool
		rm.dataModuleInstance, ok = data.New().NewModuleInstance(vu).(*data.Data)
		if !ok {
			common.Throw(vu.Runtime(), errors.New("failed to create data module instance"))
		}
	}

	return &ModuleInstance{vu: vu, RootModule: rm}
}

// Exports implements the modules.Module interface and returns the exports of
// our module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"parse":  mi.Parse,
			"Parser": mi.NewParser,
		},
	}
}

// Parser is a CSV parser.
type Parser struct {
	// currentLine holds the current line number being read by the parser.
	currentLine atomic.Int64

	// reader is the CSV reader that enables to read records from the provided
	// input file.
	reader *Reader

	// options holds the parser's as provided by the user.
	options options

	// vu is the VU instance that owns this module instance.
	vu modules.VU
}

// parseSharedArrayNamePrefix is the prefix used for the shared array names created by the Parse function.
const parseSharedArrayNamePrefix = "csv.parse."

// Parse parses the provided CSV file, and returns a promise that resolves to a shared array
// containing the parsed data.
func (mi *ModuleInstance) Parse(file sobek.Value, options sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(mi.vu)

	rt := mi.vu.Runtime()

	// 1. Make sure the Sobek object is a fs.File (Sobek operation)
	var fileObj fs.File
	if err := mi.vu.Runtime().ExportTo(file, &fileObj); err != nil {
		reject(fmt.Errorf("first argument expected to be a fs.File instance, got %T instead", file))
		return promise
	}

	parserOptions := newDefaultParserOptions()
	if options != nil {
		var err error
		parserOptions, err = newParserOptionsFrom(options.ToObject(rt))
		if err != nil {
			reject(fmt.Errorf("failed to interpret the provided Parser options; reason: %w", err))
			return promise
		}
	}

	r, err := NewReaderFrom(fileObj.ReadSeekStater, parserOptions)
	if err != nil {
		reject(fmt.Errorf("failed to create a new parser; reason: %w", err))
		return promise
	}

	go func() {
		underlyingSharedArrayName := parseSharedArrayNamePrefix + strconv.Itoa(time.Now().Nanosecond())

		// Because we rely on the data module to create the shared array, we need to
		// make sure that the data module is initialized before we can proceed, and that we don't instantiate
		// it multiple times.
		//
		// As such we hold a single instance of it in the RootModule, and we use it to create the shared array.
		resolve(mi.RootModule.dataModuleInstance.NewSharedArrayFrom(mi.vu.Runtime(), underlyingSharedArrayName, r))
	}()

	return promise
}

// NewParser creates a new CSV parser instance.
func (mi *ModuleInstance) NewParser(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()

	if mi.vu.State() != nil {
		common.Throw(rt, errors.New("csv Parser constructor must be called in the init context"))
	}

	if len(call.Arguments) < 1 || sobek.IsUndefined(call.Argument(0)) {
		common.Throw(rt, fmt.Errorf("csv Parser constructor takes at least one non-nil source argument"))
	}

	fileArg := call.Argument(0)
	if common.IsNullish(fileArg) {
		common.Throw(rt, fmt.Errorf("csv Parser constructor takes at least one non-nil source argument"))
	}

	// 1. Make sure the Sobek object is a fs.File (Sobek operation)
	var file fs.File
	if err := mi.vu.Runtime().ExportTo(fileArg, &file); err != nil {
		common.Throw(
			mi.vu.Runtime(),
			fmt.Errorf("first argument expected to be a fs.File instance, got %T instead", call.Argument(0)),
		)
	}

	options := newDefaultParserOptions()
	if len(call.Arguments) == 2 && !sobek.IsUndefined(call.Argument(1)) {
		var err error
		options, err = newParserOptionsFrom(call.Argument(1).ToObject(rt))
		if err != nil {
			common.Throw(rt, fmt.Errorf("encountered an error while interpreting Parser options; reason: %w", err))
		}
	}

	// Instantiate and configure a csv reader using the provided file and options
	r, err := NewReaderFrom(file.ReadSeekStater, options)
	if err != nil {
		common.Throw(rt, fmt.Errorf("failed to create a new parser; reason: %w", err))
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
func (p *Parser) Next() *sobek.Promise {
	promise, resolve, reject := promises.New(p.vu)

	go func() {
		var record any
		var done bool
		var err error

		record, err = p.reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				resolve(parseResult{Done: true, Value: []string{}})
				return
			}

			reject(err)
			return
		}

		p.currentLine.Add(1)

		resolve(parseResult{Done: done, Value: record})
	}()

	return promise
}

// parseResult holds the result of a CSV parser's parsing operation such
// as when calling the [Next].
type parseResult struct {
	// Done indicates whether the parser has finished reading the file.
	Done bool `js:"done"`

	// Value holds the line's records value.
	Value any `js:"value"`
}

// options holds options used to configure CSV parsing when utilizing the module.
//
// The options can be used to either configure the CSV parser, or the parse function.
// They offer to customize the behavior of the parser, such as the delimiter, whether
// to skip the first line, or  to start reading from a specific line, and stop reading
// at a specific line.
type options struct {
	// Delimiter is the character that separates the fields in the CSV.
	Delimiter rune `js:"delimiter"`

	// SkipFirstLine indicates whether the first line should be skipped.
	SkipFirstLine bool `js:"skipFirstLine"`

	// FromLine indicates the line from which to start reading the CSV file.
	FromLine null.Int `js:"fromLine"`

	// ToLine indicates the line at which to stop reading the CSV file (inclusive).
	ToLine null.Int `js:"toLine"`

	// AsObjects indicates that the CSV rows should be returned as objects, where
	// the keys are the header column names, and values are the corresponding
	// row values.
	//
	// When this option is enabled, the first line of the CSV file is treated as the header.
	//
	// If the option is set and no header line is present, this should be considered an error
	// case.
	//
	// This option is incompatible with the [SkipFirstLine] option, and if both are set, an error
	// should be returned. Same thing applies if the [FromLine] option is set to a value greater
	// than 0.
	AsObjects null.Bool `js:"asObjects"`
}

// newDefaultParserOptions creates a new options instance with default values.
func newDefaultParserOptions() options {
	return options{
		Delimiter:     ',',
		SkipFirstLine: false,
		AsObjects:     null.BoolFrom(false),
	}
}

// newParserOptions creates a new options instance from the given Sobek object.
func newParserOptionsFrom(obj *sobek.Object) (options, error) {
	options := newDefaultParserOptions()

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

	if v := obj.Get("asObjects"); v != nil {
		options.AsObjects = null.BoolFrom(v.ToBoolean())
	}

	if options.FromLine.Valid && options.ToLine.Valid && options.FromLine.Int64 >= options.ToLine.Int64 {
		return options, fmt.Errorf("fromLine must be less than or equal to toLine")
	}

	return options, nil
}
