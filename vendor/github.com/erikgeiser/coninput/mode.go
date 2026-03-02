//go:build windows
// +build windows

package coninput

import (
	"strings"

	"golang.org/x/sys/windows"
)

// AddInputModes returns the given mode with one or more additional modes enabled.
func AddInputModes(mode uint32, enableModes ...uint32) uint32 {
	for _, enableMode := range enableModes {
		mode |= enableMode
	}

	return mode
}

// RemoveInputModes returns the given mode with one or more additional modes disabled.
func RemoveInputModes(mode uint32, disableModes ...uint32) uint32 {
	for _, disableMode := range disableModes {
		mode &^= disableMode
	}

	return mode
}

// ToggleInputModes returns the given mode with one or more additional modes toggeled.
func ToggleInputModes(mode uint32, toggleModes ...uint32) uint32 {
	for _, toggeMode := range toggleModes {
		mode ^= toggeMode
	}

	return mode
}

var inputModes = []struct {
	mode uint32
	name string
}{
	{mode: windows.ENABLE_ECHO_INPUT, name: "ENABLE_ECHO_INPUT"},
	{mode: windows.ENABLE_INSERT_MODE, name: "ENABLE_INSERT_MODE"},
	{mode: windows.ENABLE_LINE_INPUT, name: "ENABLE_LINE_INPUT"},
	{mode: windows.ENABLE_MOUSE_INPUT, name: "ENABLE_MOUSE_INPUT"},
	{mode: windows.ENABLE_PROCESSED_INPUT, name: "ENABLE_PROCESSED_INPUT"},
	{mode: windows.ENABLE_QUICK_EDIT_MODE, name: "ENABLE_QUICK_EDIT_MODE"},
	{mode: windows.ENABLE_WINDOW_INPUT, name: "ENABLE_WINDOW_INPUT"},
	{mode: windows.ENABLE_VIRTUAL_TERMINAL_INPUT, name: "ENABLE_VIRTUAL_TERMINAL_INPUT"},
}

// ListInputMode returnes the isolated enabled input modes as a list.
func ListInputModes(mode uint32) []uint32 {
	modes := []uint32{}

	for _, inputMode := range inputModes {
		if mode&inputMode.mode > 0 {
			modes = append(modes, inputMode.mode)
		}
	}

	return modes
}

// ListInputMode returnes the isolated enabled input mode names as a list.
func ListInputModeNames(mode uint32) []string {
	modes := []string{}

	for _, inputMode := range inputModes {
		if mode&inputMode.mode > 0 {
			modes = append(modes, inputMode.name)
		}
	}

	return modes
}

// DescribeInputMode returns a string containing the names of each enabled input mode.
func DescribeInputMode(mode uint32) string {
	return strings.Join(ListInputModeNames(mode), "|")
}
