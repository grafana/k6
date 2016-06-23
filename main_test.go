package main

import (
	"github.com/loadimpact/speedboat/stats"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseBackendStdout(t *testing.T) {
	output, err := parseBackend("-")
	assert.NoError(t, err)
	assert.IsType(t, &stats.JSONBackend{}, output)
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
