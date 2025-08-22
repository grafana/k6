package csv

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/fsext"
)

func TestStreamingReaderBasic(t *testing.T) {
	t.Parallel()

	// Create a temporary CSV file
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	csvContent := "name,age,city\nJohn,30,NYC\nJane,25,LA\nBob,35,Chicago"
	
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	// Create filesystem
	fs := fsext.NewOsFs()

	// Test streaming reader
	options := newDefaultParserOptions()
	reader, err := NewStreamingReaderFrom(fs, csvPath, options)
	require.NoError(t, err)
	defer reader.Close()

	// Read first record
	record1, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age", "city"}, record1)

	// Read second record
	record2, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, []string{"John", "30", "NYC"}, record2)

	// Read third record
	record3, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, []string{"Jane", "25", "LA"}, record3)

	// Read fourth record
	record4, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, []string{"Bob", "35", "Chicago"}, record4)

	// Should reach EOF
	_, err = reader.Read()
	assert.True(t, errors.Is(err, io.EOF))
}

func TestStreamingReaderWithSkipFirstLine(t *testing.T) {
	t.Parallel()

	// Create a temporary CSV file
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	csvContent := "name,age,city\nJohn,30,NYC\nJane,25,LA"
	
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	// Create filesystem
	fs := fsext.NewOsFs()

	// Test streaming reader with skipFirstLine
	options := newDefaultParserOptions()
	options.SkipFirstLine = true
	
	reader, err := NewStreamingReaderFrom(fs, csvPath, options)
	require.NoError(t, err)
	defer reader.Close()

	// Should read John's record first (header skipped)
	record1, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, []string{"John", "30", "NYC"}, record1)

	// Should read Jane's record second
	record2, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, []string{"Jane", "25", "LA"}, record2)

	// Should reach EOF
	_, err = reader.Read()
	assert.True(t, errors.Is(err, io.EOF))
}

func TestStreamingReaderWithAsObjects(t *testing.T) {
	t.Parallel()

	// Create a temporary CSV file
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	csvContent := "name,age,city\nJohn,30,NYC\nJane,25,LA"
	
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	// Create filesystem
	fs := fsext.NewOsFs()

	// Test streaming reader with asObjects
	options := newDefaultParserOptions()
	options.AsObjects.Valid = true
	options.AsObjects.Bool = true
	
	reader, err := NewStreamingReaderFrom(fs, csvPath, options)
	require.NoError(t, err)
	defer reader.Close()

	// Should read John's record as object
	record1, err := reader.Read()
	require.NoError(t, err)
	expected1 := map[string]string{"name": "John", "age": "30", "city": "NYC"}
	assert.Equal(t, expected1, record1)

	// Should read Jane's record as object
	record2, err := reader.Read()
	require.NoError(t, err)
	expected2 := map[string]string{"name": "Jane", "age": "25", "city": "LA"}
	assert.Equal(t, expected2, record2)

	// Should reach EOF
	_, err = reader.Read()
	assert.True(t, errors.Is(err, io.EOF))
}

func TestStreamingReaderNonExistentFile(t *testing.T) {
	t.Parallel()

	// Create filesystem
	fs := fsext.NewOsFs()

	// Test streaming reader with non-existent file
	options := newDefaultParserOptions()
	
	_, err := NewStreamingReaderFrom(fs, "non-existent-file.csv", options)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestStreamingReaderMemoryUsage(t *testing.T) {
	t.Parallel()

	// Create a larger CSV file to test memory efficiency
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "large.csv")
	
	// Create a CSV with 1000 rows
	var csvContent string
	csvContent = "id,name,value\n"
	for i := 0; i < 1000; i++ {
		csvContent += fmt.Sprintf("%d,user%d,value%d\n", i, i, i)
	}
	
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	// Create filesystem
	fs := fsext.NewOsFs()

	// Test streaming reader
	options := newDefaultParserOptions()
	reader, err := NewStreamingReaderFrom(fs, csvPath, options)
	require.NoError(t, err)
	defer reader.Close()

	// Read all records to ensure they're processed correctly
	recordCount := 0
	for {
		_, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		recordCount++
	}

	// Should have read header + 1000 data rows
	assert.Equal(t, 1001, recordCount)
} 