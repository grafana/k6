//go:build codegen

package main

import (
	"encoding/json"
	"io"
)

func jsonGen(out io.Writer) error {
	encoder := json.NewEncoder(out)

	encoder.SetIndent("", "  ")

	return encoder.Encode(getFuncLookups())
}
