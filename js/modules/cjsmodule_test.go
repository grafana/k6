package modules

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/sobek/parser"
	"github.com/stretchr/testify/require"
)

func TestRequireCalls(t *testing.T) {
	t.Parallel()
	ts := []struct {
		script string
		list   []string
	}{
		{
			script: `require("something")`,
			list:   []string{"something"},
		},
		{
			script: `require("something"); require("else")`,
			list:   []string{"something", "else"},
		},
		{
			script: `function a() { require("something"); } require("else")`,
			list:   []string{"something", "else"},
		},
		{
			script: `export function a () { require("something"); } require("else")`,
			list:   []string{"something", "else"},
		},
		{
			script: `export const a = require("something");  require("else")`,
			list:   []string{"something", "else"},
		},
		{
			script: `var a = require("something");  require("else")`,
			list:   []string{"something", "else"},
		},
	}

	for _, test := range ts {
		t.Run(test.script, func(t *testing.T) {
			t.Parallel()
			a, err := sobek.Parse("script", test.script, parser.IsModule)
			require.NoError(t, err)

			list := findRequireFunctionInAST(a.Body)
			require.EqualValues(t, test.list, list)
		})
	}
}
