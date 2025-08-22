package csv

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/spf13/afero"
	"go.k6.io/k6/internal/js/modules/k6/data"
	"go.k6.io/k6/lib/fsext"
)

// StreamingReader is a CSV reader that streams data without loading the entire file into memory.
//
// Unlike the current Reader implementation, this one opens the file directly from the filesystem
// and uses a buffered reader to process CSV records one at a time, avoiding memory issues with large files.
type StreamingReader struct {
	csvReader *csv.Reader
	file      afero.File
	
	// currentLine tracks the current line number.
	currentLine atomic.Int64
	
	// options holds the reader's options.
	options options
	
	// columnNames stores the column names when the asObjects option is enabled
	// in order to be able to map each row values to their corresponding column.
	columnNames []string
	
	// initialized tracks whether the reader has been properly initialized
	initialized bool
}

// NewStreamingReaderFrom creates a new streaming CSV reader from the provided file path.
//
// Instead of loading the entire file into memory like the current implementation,
// this opens the file with a small buffer and processes it line by line.
func NewStreamingReaderFrom(fs fsext.Fs, filePath string, options options) (*StreamingReader, error) {
	if err := validateOptions(options); err != nil {
		return nil, err
	}

	if options.Delimiter == 0 {
		options.Delimiter = ','
	}

	// Open the file directly from the filesystem without loading it into memory
	file, err := fs.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	// Create a buffered reader to efficiently read the file in small chunks
	bufferedReader := bufio.NewReaderSize(file, 64*1024) // 64KB buffer
	csvReader := csv.NewReader(bufferedReader)
	csvReader.Comma = options.Delimiter

	reader := &StreamingReader{
		csvReader: csvReader,
		file:      file,
		options:   options,
	}

	// Initialize the reader (handle header, skip lines, etc.)
	if err := reader.initialize(); err != nil {
		file.Close() // Clean up on error
		return nil, err
	}

	return reader, nil
}

// initialize handles the initial setup of the reader (reading headers, skipping lines, etc.)
func (r *StreamingReader) initialize() error {
	if r.initialized {
		return nil
	}

	asObjectsEnabled := r.options.AsObjects.Valid && r.options.AsObjects.Bool
	if asObjectsEnabled {
		header, err := r.csvReader.Read()
		if err != nil {
			return fmt.Errorf("failed to read the first line; reason: %w", err)
		}
		r.columnNames = header
		r.currentLine.Add(1)
	}

	if r.options.SkipFirstLine && (!r.options.FromLine.Valid || r.options.FromLine.Int64 == 0) {
		if _, err := r.csvReader.Read(); err != nil {
			return fmt.Errorf("failed to skip the first line; reason: %w", err)
		}
		r.currentLine.Add(1)
	}

	if r.options.FromLine.Valid && r.options.FromLine.Int64 > 0 {
		for r.currentLine.Load() < r.options.FromLine.Int64 {
			if _, err := r.csvReader.Read(); err != nil {
				return fmt.Errorf("failed to skip lines until line %d; reason: %w", r.options.FromLine.Int64, err)
			}
			r.currentLine.Add(1)
		}
	}

	r.initialized = true
	return nil
}

// Read reads a record from the CSV file using streaming approach.
func (r *StreamingReader) Read() (any, error) {
	toLineSet := r.options.ToLine.Valid

	// If the `toLine` option was set and we have reached it, we return EOF.
	if toLineSet && r.options.ToLine.Int64 > 0 && r.currentLine.Load() > r.options.ToLine.Int64 {
		return nil, io.EOF
	}

	record, err := r.csvReader.Read()
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

// Close closes the underlying file.
func (r *StreamingReader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// Ensure StreamingReader implements the RecordReader interface
var _ data.RecordReader = (*StreamingReader)(nil) 