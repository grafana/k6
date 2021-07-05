// blatantly stolen from  https://github.com/mostafa/goja_debugger

package js

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

const (
	GreenColor = "\u001b[32m"
	GrayColor  = "\u001b[38;5;245m"
	ResetColor = "\u001b[0m"
)

type Command struct {
	Name string
	Args []string
}

func parseCmd(userInput string) (*Command, error) {
	data := strings.Split(userInput, " ")
	if len(data) == 0 {
		return nil, errors.New("unknown command")
	}
	name := data[0]
	var args []string
	if len(data) > 1 {
		args = append(args, data[1:]...)
	}
	return &Command{Name: name, Args: args}, nil
}

func repl(runtime *goja.Runtime, dbg *goja.Debugger, userInput string, getsrc func(string) ([]byte, error)) bool {
	cmd, err := parseCmd(userInput)
	if err != nil {
		fmt.Println(err.Error())
		return true
	}

	switch cmd.Name {
	case "setBreakpoint", "sb":
		if len(cmd.Args) < 2 {
			fmt.Println("sb filename linenumber")
			return true
		}
		if line, err := strconv.Atoi(cmd.Args[1]); err != nil {
			fmt.Printf("Cannot convert %s to line number\n", cmd.Args[1])
		} else {
			err := dbg.SetBreakpoint(cmd.Args[0], line)
			if err != nil {
				fmt.Println(err.Error())
			}
		}
	case "clearBreakpoint", "cb":
		if len(cmd.Args) < 2 {
			fmt.Println("cb filename linenumber")
			return true
		}
		if line, err := strconv.Atoi(cmd.Args[1]); err != nil {
			fmt.Printf("Cannot convert %s to line number\n", cmd.Args[1])
		} else {
			err := dbg.ClearBreakpoint(cmd.Args[0], line)
			if err != nil {
				fmt.Println(err.Error())
			}
		}
	case "breakpoints", "b":
		breakpoints, err := dbg.Breakpoints()
		if err != nil {
			fmt.Println(err.Error())
		} else {
			for filename := range breakpoints {
				fmt.Printf("Breakpoint on %s:%v\n", filename, breakpoints[filename])
			}
		}
	case "run", "r":
		// Works like continue, only if the activation reason is program start ("s")
		if dbg.PC() == 0 {
			return false
		} else {
			fmt.Println("Error: only works if program is not started")
		}
	case "next", "n":
		err = dbg.Next()
		if err != nil {
			// fmt.Println(err.Error())
			return false
		}
	case "cont", "continue", "c":
		return false
	case "step", "s":
		err = dbg.StepIn()
		if err != nil {
			return false
		}
	case "exec", "e":
		val, err := dbg.Exec(strings.Join(cmd.Args, " "))
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			break
		}
		fmt.Printf("< %s\n", val)
	case "print", "p":
		val, err := dbg.Print(strings.Join(cmd.Args, ""))
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			break
		}
		fmt.Printf("< %s\n", val)
	case "list", "l":
		b, err := getsrc(dbg.Filename())
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			break
		}
		lines := strings.Split(string(b), "\n")
		currentLine := dbg.Line()
		lineIndex := currentLine - 1
		var builder strings.Builder
		for idx, lineContents := range lines {
			if inRange(lineIndex, idx-4, idx+4) {
				lineNumber := idx + 1
				totalPadding := 6
				digitCount := countDigits(lineNumber)
				if digitCount >= totalPadding {
					totalPadding = digitCount + 1
				}
				if currentLine == lineNumber {
					padding := strings.Repeat(" ", totalPadding-digitCount)
					builder.Write([]byte(fmt.Sprintf("%s>%s %d%s%s\n", GreenColor, ResetColor, currentLine, padding, lines[lineIndex])))
				} else {
					padding := strings.Repeat(" ", totalPadding-digitCount)
					builder.Write([]byte(fmt.Sprintf("%s  %d%s%s%s\n", GrayColor, lineNumber, padding, lineContents, ResetColor)))
				}
			}
		}
		fmt.Println(builder.String())
	case "backtrace", "bt":
		stack := runtime.CaptureCallStack(0, nil)
		var backtrace bytes.Buffer
		backtrace.WriteRune('\n')
		for _, frame := range stack {
			frame.Write(&backtrace)
			backtrace.WriteRune('\n')
		}
		fmt.Println(backtrace.String())
	case "help", "h":
		fmt.Println(help)
	case "quit", "q":
		os.Exit(0)
	default:
		fmt.Printf("Unknown command, `%s`. You can use `h` to print available commands\n", userInput)
	}

	return true
}

func inRange(i, min, max int) bool {
	if (i >= min) && (i <= max) {
		return true
	} else {
		return false
	}
}

func countDigits(number int) int {
	if number < 10 {
		return 1
	} else {
		return 1 + countDigits(number/10)
	}
}

var help = `
setBreakpoint, sb        Set a breakpoint on a given file and line
clearBreakpoint, cb      Clear a breakpoint on a given file and line
breakpoints, b           List all known breakpoints
run, r                   Run program until a breakpoint/debugger statement if program is not started
next, n                  Continue to next line in current file
cont, c                  Resume execution until next debugger line
step, s                  Step into, potentially entering a function
out, o                   Step out, leaving the current function (not implemented yet)
exec, e                  Evaluate the expression and print the value
list, l                  Print the source around the current line where execution is currently paused
print, p                 Print the provided variable's value
backtrace, bt            Print the current backtrace
help, h                  Print this very help message
quit, q                  Exit debugger and quit (Ctrl+C)
`[1:] // this removes the first new line

func printDebuggingReason(dbg *goja.Debugger, reason goja.ActivationReason) {
	if reason == goja.ProgramStartActivation {
		fmt.Printf("Break on start in %s:%d\n", dbg.Filename(), dbg.Line())
	} else if reason == goja.BreakpointActivation {
		fmt.Printf("Break on breakpoint in %s:%d\ns", dbg.Filename(), dbg.Line())
	} else {
		fmt.Printf("Break on debugger statement in %s:%d\n", dbg.Filename(), dbg.Line())
	}
}

func getInfo(dbg *goja.Debugger) string {
	return fmt.Sprintf("[%d]", dbg.PC())
}
