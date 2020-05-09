package main

import "strings"

type leftpad struct{}

func (*leftpad) Leftpad(s string, l int, padding string) string {
	if len(s) >= l {
		return s
	}

	return strings.Repeat(padding, l-len(s)) + s
}

type plugin struct{}

var JavaScriptPlugin plugin

func (*plugin) Name() string {
	return "Leftpad"
}

func (*plugin) Setup() error {
	return nil
}

func (*plugin) Teardown() error {
	return nil
}

func (*plugin) GetModules() map[string]interface{} {
	mods := map[string]interface{}{
		"leftpad": &leftpad{},
	}
	return mods
}

func init() {
	JavaScriptPlugin = plugin{}
	_ = JavaScriptPlugin // shut up, linter...
}
