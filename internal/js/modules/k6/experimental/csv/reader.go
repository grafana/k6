package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"sync/atomic"

	"go.k6.io/k6/internal/js/modules/k6/data"
)

// Reader is a CSV reader.
//
// It wraps a csv.Reader and provides additional functionality such as the ability to stop reading at a specific line.
type Reader struct {
	csv *csv.Reader

	// currentLine tracks the current line number.
	currentLine atomic.Int64

	// options holds the reader's options.
	options options

	// columnNames stores the column names when the asObjects option is enabled
	// in order to be able to map each row values to their corresponding column.
	columnNames []string
}

// NewReaderFrom creates a new CSV reader from the provided io.Reader.
//
// It will check whether the first line should be skipped and consume it if necessary.
// It will also check whether the reader should start from a specific line and skip lines until that line is reached.
// We perform these operations here to avoid having to do them in the Read method.
//
// Hence, this constructor function can return an error if the first line cannot be skipped or if the reader
// cannot start from the specified line.
func NewReaderFrom(r io.Reader, options options) (*Reader, error) {
	if r == nil {
		return nil, fmt.Errorf("the reader cannot be nil")
	}

	if err := validateOptions(options); err != nil {
		return nil, err
	}

	if options.Delimiter == 0 {
		options.Delimiter = ','
	}

	csvParser := csv.NewReader(r)
	csvParser.Comma = options.Delimiter

	reader := &Reader{
		csv:     csvParser,
		options: options,
	}

	asObjectsEnabled := options.AsObjects.Valid && options.AsObjects.Bool
	if asObjectsEnabled {
		header, err := csvParser.Read()
		if err != nil {
			return nil, fmt.Errorf("failed to read the first line; reason: %w", err)
		}
		reader.columnNames = header
		reader.currentLine.Add(1)
	}

	if options.SkipFirstLine && (!options.FromLine.Valid || options.FromLine.Int64 == 0) {
		if _, err := csvParser.Read(); err != nil {
			return nil, fmt.Errorf("failed to skip the first line; reason: %w", err)
		}

		reader.currentLine.Add(1)
	}

	if options.FromLine.Valid && options.FromLine.Int64 > 0 {
		for reader.currentLine.Load() < options.FromLine.Int64 {
			if _, err := csvParser.Read(); err != nil {
				return nil, fmt.Errorf("failed to skip lines until line %d; reason: %w", options.FromLine.Int64, err)
			}
			reader.currentLine.Add(1)
		}
	}

	return reader, nil
}

// The csv module's read must implement the RecordReader interface.
var _ data.RecordReader = (*Reader)(nil)

// Read reads a record from the CSV file.
//
// If the `header` option is enabled, it will return a map of the record.
// Otherwise, it will return the record as a slice of strings.
func (r *Reader) Read() (any, error) {
	toLineSet := r.options.ToLine.Valid

	// If the `toLine` option was set and we have reached it, we return EOF.
	if toLineSet && r.options.ToLine.Int64 > 0 && r.currentLine.Load() > r.options.ToLine.Int64 {
		return nil, io.EOF
	}

	record, err := r.csv.Read()
	if err != nil {
		return nil, err
	}

	r.currentLine.Add(1)

	// If header option is enabled, return a map of the record.
	if r.options.AsObjects.Valid && r.options.AsObjects.Bool {
		if r.columnNames == nil {
			return nil, fmt.Errorf("the 'asObjects' option is enabled, but no header was found")
		}

		if len(record) != len(r.columnNames) {
			return nil, fmt.Errorf("record length (%d) doesn't match header length (%d)", len(record), len(r.columnNames))
		}

		recordMap := make(map[string]string)
		for i, value := range record {
			recordMap[r.columnNames[i]] = value
		}

		return recordMap, nil
	}

	return record, nil
}

// validateOptions validates the reader options and returns an error if any validation fails.
func validateOptions(options options) error {
	var (
		fromLineSet      = options.FromLine.Valid
		toLineSet        = options.ToLine.Valid
		skipFirstLineSet = options.SkipFirstLine
		asObjectsEnabled = options.AsObjects.Valid && options.AsObjects.Bool
	)

	if asObjectsEnabled && skipFirstLineSet {
		return fmt.Errorf("the 'header' option cannot be enabled when 'skipFirstLine' is true")
	}

	if asObjectsEnabled && fromLineSet && options.FromLine.Int64 > 0 {
		return fmt.Errorf("the 'header' option cannot be enabled when 'fromLine' is set to a value greater than 0")
	}

	if fromLineSet && options.FromLine.Int64 < 0 {
		return fmt.Errorf("the 'fromLine' option must be greater than or equal to 0; got %d", options.FromLine.Int64)
	}

	if toLineSet && options.ToLine.Int64 < 0 {
		return fmt.Errorf("the 'toLine' option must be greater than or equal to 0; got %d", options.ToLine.Int64)
	}

	if fromLineSet && toLineSet && options.FromLine.Int64 >= options.ToLine.Int64 {
		return fmt.Errorf(
			"the 'fromLine' option must be less than the 'toLine' option; got 'fromLine': %d, 'toLine': %d",
			options.FromLine.Int64, options.ToLine.Int64,
		)
	}

	return nil
}
