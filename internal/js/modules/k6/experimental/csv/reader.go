package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"sync/atomic"
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

	// Ensure the default delimiter is set.
	if options.Delimiter == 0 {
		options.Delimiter = ','
	}

	csvParser := csv.NewReader(r)
	csvParser.Comma = options.Delimiter

	reader := &Reader{
		csv:     csvParser,
		options: options,
	}

	var (
		fromLineSet        = options.FromLine.Valid
		toLineSet          = options.ToLine.Valid
		skipFirstLineSet   = options.SkipFirstLine
		fromLineIsPositive = fromLineSet && options.FromLine.Int64 >= 0
		toLineIsPositive   = toLineSet && options.ToLine.Int64 >= 0
	)

	// If set, the fromLine option should either be greater than or equal to 0.
	if fromLineSet && !fromLineIsPositive {
		return nil, fmt.Errorf("the 'fromLine' option must be greater than or equal to 0; got %d", options.FromLine.Int64)
	}

	// If set, the toLine option should be strictly greater than or equal to 0.
	if toLineSet && !toLineIsPositive {
		return nil, fmt.Errorf("the 'toLine' option must be greater than or equal to 0; got %d", options.ToLine.Int64)
	}

	// if the `fromLine` and `toLine` options are set, and `fromLine` is greater or equal to `toLine`, we return an error.
	if fromLineSet && toLineSet && options.FromLine.Int64 >= options.ToLine.Int64 {
		return nil, fmt.Errorf(
			"the 'fromLine' option must be less than the 'toLine' option; got 'fromLine': %d, 'toLine': %d",
			options.FromLine.Int64, options.ToLine.Int64,
		)
	}

	// If the user wants to skip the first line, we consume and discard it.
	if skipFirstLineSet && (!fromLineSet || options.FromLine.Int64 == 0) {
		_, err := csvParser.Read()
		if err != nil {
			return nil, fmt.Errorf("failed to skip the first line; reason: %w", err)
		}

		reader.currentLine.Add(1)
	}

	if fromLineSet && options.FromLine.Int64 > 0 {
		// We skip lines until we reach the specified line.
		for reader.currentLine.Load() < options.FromLine.Int64 {
			_, err := csvParser.Read()
			if err != nil {
				return nil, fmt.Errorf("failed to skip lines until line %d; reason: %w", options.FromLine.Int64, err)
			}
			reader.currentLine.Add(1)
		}
	}

	return reader, nil
}

func (r *Reader) Read() ([]string, error) {
	toLineSet := r.options.ToLine.Valid

	// If the `toLine` option was set and we have reached it, we return EOF.
	if toLineSet && r.options.ToLine.Int64 > 0 && r.currentLine.Load() > r.options.ToLine.Int64 {
		return nil, io.EOF
	}

	records, err := r.csv.Read()
	if err != nil {
		return nil, err
	}

	r.currentLine.Add(1)

	return records, nil
}
