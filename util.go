package main

import (
	"gopkg.in/guregu/null.v3"
	"gopkg.in/urfave/cli.v1"
)

// cliBool returns a CLI argument as a bool, which is invalid if not given.
func cliBool(cc *cli.Context, name string) null.Bool {
	return null.NewBool(cc.Bool(name), cc.IsSet(name))
}

// cliInt64 returns a CLI argument as an int64, which is invalid if not given.
func cliInt64(cc *cli.Context, name string) null.Int {
	return null.NewInt(cc.Int64(name), cc.IsSet(name))
}

// cliFloat64 returns a CLI argument as a float64, which is invalid if not given.
func cliFloat64(cc *cli.Context, name string) null.Float {
	return null.NewFloat(cc.Float64(name), cc.IsSet(name))
}

// cliDuration returns a CLI argument as a duration string, which is invalid if not given.
func cliDuration(cc *cli.Context, name string) null.String {
	return null.NewString(cc.Duration(name).String(), cc.IsSet(name))
}
