//go:build windows
// +build windows

package coninput

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procReadConsoleInputW             = modkernel32.NewProc("ReadConsoleInputW")
	procPeekConsoleInputW             = modkernel32.NewProc("PeekConsoleInputW")
	procGetNumberOfConsoleInputEvents = modkernel32.NewProc("GetNumberOfConsoleInputEvents")
	procFlushConsoleInputBuffer       = modkernel32.NewProc("FlushConsoleInputBuffer")
)

// NewStdinHandle is a shortcut for windows.GetStdHandle(windows.STD_INPUT_HANDLE).
func NewStdinHandle() (windows.Handle, error) {
	return windows.GetStdHandle(windows.STD_INPUT_HANDLE)
}

// WinReadConsoleInput is a thin wrapper around the Windows console API function
// ReadConsoleInput (see
// https://docs.microsoft.com/en-us/windows/console/readconsoleinput). In most
// cases it is more practical to either use ReadConsoleInput or
// ReadNConsoleInputs.
func WinReadConsoleInput(consoleInput windows.Handle, buffer *InputRecord,
	length uint32, numberOfEventsRead *uint32) error {
	r, _, e := syscall.Syscall6(procReadConsoleInputW.Addr(), 4,
		uintptr(consoleInput), uintptr(unsafe.Pointer(buffer)), uintptr(length),
		uintptr(unsafe.Pointer(numberOfEventsRead)), 0, 0)
	if r == 0 {
		return error(e)
	}

	return nil
}

// ReadNConsoleInputs is a wrapper around ReadConsoleInput (see
// https://docs.microsoft.com/en-us/windows/console/readconsoleinput) that
// automates the event buffer allocation in oder to provide io.Reader-like
// sematics. maxEvents must be greater than zero.
func ReadNConsoleInputs(console windows.Handle, maxEvents uint32) ([]InputRecord, error) {
	if maxEvents == 0 {
		return nil, fmt.Errorf("maxEvents cannot be zero")
	}

	var inputRecords = make([]InputRecord, maxEvents)
	n, err := ReadConsoleInput(console, inputRecords)

	return inputRecords[:n], err
}

// ReadConsoleInput provides an ideomatic interface to the Windows console API
// function ReadConsoleInput (see
// https://docs.microsoft.com/en-us/windows/console/readconsoleinput). The size
// of inputRecords must be greater than zero.
func ReadConsoleInput(console windows.Handle, inputRecords []InputRecord) (uint32, error) {
	if len(inputRecords) == 0 {
		return 0, fmt.Errorf("size of input record buffer cannot be zero")
	}

	var read uint32
	err := WinReadConsoleInput(console, &inputRecords[0], uint32(len(inputRecords)), &read)

	return read, err
}

// WinPeekConsoleInput is a thin wrapper around the Windows console API function
// PeekConsoleInput (see
// https://docs.microsoft.com/en-us/windows/console/peekconsoleinput). In most
// cases it is more practical to either use PeekConsoleInput or
// PeekNConsoleInputs.
func WinPeekConsoleInput(consoleInput windows.Handle, buffer *InputRecord,
	length uint32, numberOfEventsRead *uint32) error {
	r, _, e := syscall.Syscall6(procPeekConsoleInputW.Addr(), 4,
		uintptr(consoleInput), uintptr(unsafe.Pointer(buffer)), uintptr(length),
		uintptr(unsafe.Pointer(numberOfEventsRead)), 0, 0)
	if r == 0 {
		return error(e)
	}

	return nil

}

// PeekNConsoleInputs is a wrapper around PeekConsoleInput (see
// https://docs.microsoft.com/en-us/windows/console/peekconsoleinput) that
// automates the event buffer allocation in oder to provide io.Reader-like
// sematics. maxEvents must be greater than zero.
func PeekNConsoleInputs(console windows.Handle, maxEvents uint32) ([]InputRecord, error) {
	if maxEvents == 0 {
		return nil, fmt.Errorf("maxEvents cannot be zero")
	}

	var inputRecords = make([]InputRecord, maxEvents)
	n, err := PeekConsoleInput(console, inputRecords)

	return inputRecords[:n], err
}

// PeekConsoleInput provides an ideomatic interface to the Windows console API
// function PeekConsoleInput (see
// https://docs.microsoft.com/en-us/windows/console/peekconsoleinput). The size
// of inputRecords must be greater than zero.
func PeekConsoleInput(console windows.Handle, inputRecords []InputRecord) (uint32, error) {
	if len(inputRecords) == 0 {
		return 0, fmt.Errorf("size of input record buffer cannot be zero")
	}

	var read uint32

	err := WinPeekConsoleInput(console, &inputRecords[0], uint32(len(inputRecords)), &read)

	return read, err
}

// WinGetNumberOfConsoleInputEvents provides an ideomatic interface to the
// Windows console API function GetNumberOfConsoleInputEvents (see
// https://docs.microsoft.com/en-us/windows/console/getnumberofconsoleinputevents).
func WinGetNumberOfConsoleInputEvents(consoleInput windows.Handle, numberOfEvents *uint32) error {
	r, _, e := syscall.Syscall6(procGetNumberOfConsoleInputEvents.Addr(), 2,
		uintptr(consoleInput), uintptr(unsafe.Pointer(numberOfEvents)), 0,
		0, 0, 0)
	if r == 0 {
		return error(e)
	}

	return nil
}

// GetNumberOfConsoleInputEvents provides an ideomatic interface to the Windows
// console API function GetNumberOfConsoleInputEvents (see
// https://docs.microsoft.com/en-us/windows/console/getnumberofconsoleinputevents).
func GetNumberOfConsoleInputEvents(console windows.Handle) (uint32, error) {
	var nEvents uint32
	err := WinGetNumberOfConsoleInputEvents(console, &nEvents)

	return nEvents, err
}

func FlushConsoleInputBuffer(consoleInput windows.Handle) error {
	r, _, e := syscall.Syscall(procFlushConsoleInputBuffer.Addr(), 1, uintptr(consoleInput), 0, 0)
	if r == 0 {
		return error(e)
	}

	return nil
}
