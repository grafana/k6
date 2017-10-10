/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package ui

import (
	"io"
	"strings"

	"github.com/zyedidia/highlight"
	"gopkg.in/yaml.v2"
)

// Source: https://github.com/zyedidia/highlight/blob/master/syntax_files/yaml.yaml
var yamlSyntax *highlight.Def
var yamlSyntaxSrc = `
filetype: yaml

detect:
    filename: "\\.ya?ml$"
    header: "%YAML"

rules:
    - type: "(^| )!!(binary|bool|float|int|map|null|omap|seq|set|str) "
    - constant:  "\\b(YES|yes|Y|y|ON|on|NO|no|N|n|OFF|off)\\b"
    - constant: "\\b(true|false)\\b"
    - statement: "(:[[:space:]]|\\[|\\]|:[[:space:]]+[|>]|^[[:space:]]*- )"
    - identifier: "[[:space:]][\\*&][A-Za-z0-9]+"
    - type: "[-.\\w]+:"
    - statement: ":"
    - special:  "(^---|^\\.\\.\\.|^%YAML|^%TAG)"

    - constant.string:
        start: "\""
        end: "\""
        skip: "\\\\."
        rules:
            - constant.specialChar: "\\\\."

    - constant.string:
        start: "'"
        end: "'"
        skip: "\\\\."
        rules:
            - constant.specialChar: "\\\\."

    - comment:
        start: "#"
        end: "$"
        rules: []`[1:]

func init() {
	def, err := highlight.ParseDef([]byte(yamlSyntaxSrc))
	if err != nil {
		panic(err)
	}
	yamlSyntax = def
}

func Dump(w io.Writer, v interface{}) {
	data, err := yaml.Marshal(v)
	if err != nil {
		_, _ = ErrorColor.Fprint(w, err)
	}
	str := string(data)

	color := StdColor
	matches := highlight.NewHighlighter(yamlSyntax).HighlightString(str)
	var chunk []rune
	for i, line := range strings.Split(str, "\n") {
		match := matches[i]
		for pos, c := range line {
			if g, ok := match[pos]; ok {
				_, _ = color.Fprint(w, string(chunk))
				chunk = nil

				gs := highlight.Groups
				switch g {
				case gs["type"]:
					color = TypeColor
				case gs["comment"]:
					color = CommentColor
				case gs["constant"], gs["constant.string"]:
					color = ValueColor
				default:
					color = StdColor
				}
			}
			chunk = append(chunk, c)
		}
		chunk = append(chunk, '\n')
	}
	_, _ = color.Fprint(w, string(chunk[:len(chunk)-1]))
}
