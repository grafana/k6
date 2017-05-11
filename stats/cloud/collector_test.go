package cloud

import (
	"os"
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
)

func TestGetName(t *testing.T) {
	nameTests := []struct {
		lib      *lib.SourceData
		conf     loadimpactConfig
		expected string
	}{
		{&lib.SourceData{Filename: ""}, loadimpactConfig{}, TestName},
		{&lib.SourceData{Filename: "-"}, loadimpactConfig{}, TestName},
		{&lib.SourceData{Filename: "script.js"}, loadimpactConfig{}, "script.js"},
		{&lib.SourceData{Filename: "/file/name.js"}, loadimpactConfig{}, "name.js"},
		{&lib.SourceData{Filename: "/file/name"}, loadimpactConfig{}, "name"},
		{&lib.SourceData{Filename: "/file/name"}, loadimpactConfig{Name: "confName"}, "confName"},
	}

	for _, test := range nameTests {
		actual := getName(test.lib, test.conf)
		assert.Equal(t, actual, test.expected)
	}

	err := os.Setenv("K6CLOUD_NAME", "envname")
	assert.Nil(t, err)

	for _, test := range nameTests {
		actual := getName(test.lib, test.conf)
		assert.Equal(t, actual, "envname")

	}
}
