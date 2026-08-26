// Package command offers utility functions for testing commands
package command

import "strings"

func isFlag(s string) bool {
	return strings.HasPrefix(s, "-") || strings.HasPrefix(s, "--")
}

func parseCmd(cmd string) ([]string, map[string]string) {
	opts := []string{}
	flags := map[string]string{}

	tokens := strings.Split(cmd, " ")
	for i := 0; i < len(tokens); i++ {
		// skip empty tokens produced by repeated spaces
		if tokens[i] == "" {
			continue
		}

		if isFlag(tokens[i]) {
			if next := i + 1; next < len(tokens) && !isFlag(tokens[next]) {
				flags[tokens[i]] = tokens[next]
				i++
			} else {
				flags[tokens[i]] = ""
			}
		} else {
			opts = append(opts, tokens[i])
		}
	}

	return opts, flags
}

// AssertCmdEquals asserts if two commands have the same options according to the following rules:
// - Flags can appear in any order.
// - Arguments must appear in the same order.
// - Flags and arguments can be mixed.
//
// Examples:
// expected                        actual                   result
// ---------------------------------------------------------------
// cmd subcmd -a a -b b arg     cmd subcmd -a a -b b arg    true
// cmd subcmd -a a -b b arg     cmd subcmd -b b -a a arg    true
// cmd subcmd -a a -b b arg     cmd -a a -b b subcmd arg    true
// cmd subcmd -a a -b b arg     cmd -a a -b b arg subcmd    false
func AssertCmdEquals(expected, actual string) bool {
	eOpts, eFlags := parseCmd(expected)
	aOpts, aFlags := parseCmd(actual)

	if len(eOpts) != len(aOpts) || len(eFlags) != len(aFlags) {
		return false
	}

	for i := range eOpts {
		if eOpts[i] != aOpts[i] {
			return false
		}
	}

	for f, v := range eFlags {
		if aFlags[f] != v {
			return false
		}
	}

	return true
}
