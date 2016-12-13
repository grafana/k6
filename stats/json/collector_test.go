package json

import (
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

func TestNewWithInaccessibleFilename(t *testing.T) {
	u, _ := url.Parse("/this_should_not_exist/badplacetolog.log")
	collector, err := New(u)
	assert.NotEqual(t, err, error(nil))
	assert.Equal(t, collector, (*Collector)(nil))
}

func TestNewWithFileURL(t *testing.T) {
	u, _ := url.Parse("file:///tmp/okplacetolog.log")
	collector, err := New(u)
	assert.Equal(t, err, error(nil))
	assert.NotEqual(t, collector, (*Collector)(nil))
}

func TestNewWithFileName(t *testing.T) {
	u, _ := url.Parse("/tmp/okplacetolog.log")
	collector, err := New(u)
	assert.Equal(t, err, error(nil))
	assert.NotEqual(t, collector, (*Collector)(nil))
}

func TestNewWithLocalFilename1(t *testing.T) {
	u, _ := url.Parse("./okplacetolog.log")
	collector, err := New(u)
	assert.Equal(t, err, error(nil))
	assert.NotEqual(t, collector, (*Collector)(nil))
}

func TestNewWithLocalFilename2(t *testing.T) {
	u, _ := url.Parse("okplacetolog.log")
	collector, err := New(u)
	assert.Equal(t, err, error(nil))
	assert.NotEqual(t, collector, (*Collector)(nil))
}
