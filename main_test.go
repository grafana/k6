package main

import (
	"github.com/loadimpact/speedboat/sampler/stream"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseOutputStdoutJSON(t *testing.T) {
	output, err := parseOutput("-", "json")
	assert.NoError(t, err)
	assert.IsType(t, &stream.JSONOutput{}, output)
}

func TestParseOutputStdoutCSV(t *testing.T) {
	output, err := parseOutput("-", "csv")
	assert.NoError(t, err)
	assert.IsType(t, &stream.CSVOutput{}, output)
}

func TestParseOutputStdoutUnknown(t *testing.T) {
	_, err := parseOutput("-", "not a real format")
	assert.Error(t, err)
}

func TestGuessTypeURL(t *testing.T) {
	assert.Equal(t, typeURL, guessType("http://example.com/"))
}

func TestGuessTypeJS(t *testing.T) {
	assert.Equal(t, typeJS, guessType("script.js"))
}

func TestGuessTypeUnknown(t *testing.T) {
	assert.Equal(t, "", guessType("script.txt"))
}
