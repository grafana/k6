# Highlight
[![Go Report Card](https://goreportcard.com/badge/github.com/zyedidia/highlight)](https://goreportcard.com/report/github.com/zyedidia/highlight)
[![GoDoc](https://godoc.org/github.com/zyedidia/highlight?status.svg)](http://godoc.org/github.com/zyedidia/highlight)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/zyedidia/highlight/blob/master/LICENSE)

This is a package for syntax highlighting a large number of different languages. To see the list of
languages currently supported, see the [`syntax_files`](./syntax_files) directory.

Highlight allows you to pass in a string and get back all the information you need to syntax highlight
that string well.

This project is still a work in progress and more features and documentation will be coming later.

# Installation

```
go get github.com/zyedidia/highlight
```

# Usage

Here is how to use this package to highlight a string. We will also be using `github.com/fatih/color` to actually
colorize the output to the console.

```go
package main

import (
    "fmt"
    "io/ioutil"
    "strings"

    "github.com/fatih/color"
    "github.com/zyedidia/highlight"
)

func main() {
    // Here is the go code we will highlight
    inputString := `package main

import "fmt"

// A hello world program
func main() {
    fmt.Println("Hello world")
}`

    // Load the go syntax file
    // Make sure that the syntax_files directory is in the current directory
    syntaxFile, _ := ioutil.ReadFile("highlight/syntax_files/go.yaml")

    // Parse it into a `*highlight.Def`
    syntaxDef, err := highlight.ParseDef(syntaxFile)
    if err != nil {
        fmt.Println(err)
        return
    }

    // Make a new highlighter from the definition
    h := highlight.NewHighlighter(syntaxDef)
    // Highlight the string
    // Matches is an array of maps which point to groups
    // matches[lineNum][colNum] will give you the change in group at that line and column number
    // Note that there is only a group at a line and column number if the syntax highlighting changed at that position
    matches := h.HighlightString(inputString)

    // We split the string into a bunch of lines
    // Now we will print the string
    lines := strings.Split(inputString, "\n")
    for lineN, l := range lines {
        for colN, c := range l {
            // Check if the group changed at the current position
            if group, ok := matches[lineN][colN]; ok {
                // Check the group name and set the color accordingly (the colors chosen are arbitrary)
                if group == highlight.Groups["statement"] {
                    color.Set(color.FgGreen)
                } else if group == highlight.Groups["preproc"] {
                    color.Set(color.FgHiRed)
                } else if group == highlight.Groups["special"] {
                    color.Set(color.FgBlue)
                } else if group == highlight.Groups["constant.string"] {
                    color.Set(color.FgCyan)
                } else if group == highlight.Groups["constant.specialChar"] {
                    color.Set(color.FgHiMagenta)
                } else if group == highlight.Groups["type"] {
                    color.Set(color.FgYellow)
                } else if group == highlight.Groups["constant.number"] {
                    color.Set(color.FgCyan)
                } else if group == highlight.Groups["comment"] {
                    color.Set(color.FgHiGreen)
                } else {
                    color.Unset()
                }
            }
            // Print the character
            fmt.Print(string(c))
        }
        // This is at a newline, but highlighting might have been turned off at the very end of the line so we should check that.
        if group, ok := matches[lineN][len(l)]; ok {
            if group == highlight.Groups["default"] || group == highlight.Groups[""] {
                color.Unset()
            }
        }

        fmt.Print("\n")
    }
}
```

If you would like to automatically detect the filetype of a file based on the filename, and have the appropriate definition returned,
you can use the `DetectFiletype` function:

```go
// Name of the file
filename := ...
// The first line of the file (needed to check the filetype by header: e.g. `#!/bin/bash` means shell)
firstLine := ...

// Parse all the syntax files in an array with type []*highlight.Def
var defs []*highlight.Def
...

def := highlight.DetectFiletype(defs, filename, firstLine)
fmt.Println("Filetype is", def.FileType)
```

For a full example, see the [`syncat`](./examples) example which acts like cat but will syntax highlight the output (if highlight recognizes the filetype).
