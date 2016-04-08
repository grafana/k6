package testutil

import (
	"github.com/codegangsta/cli"
	"testing"
)

func TestWithAppContext(t *testing.T) {
	cmd := cli.Command{
		Flags: []cli.Flag{
			cli.StringFlag{Name: "arg"},
		},
	}
	err := WithAppContext("--arg value", cmd, func(c *cli.Context) {
		v := c.String("arg")
		if v != "value" {
			t.Error("Wrong value:", v)
		}
	})
	if err != nil {
		t.Error(err)
	}
}
