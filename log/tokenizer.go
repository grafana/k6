/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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

package log

import "fmt"

type token struct {
	key, value string
	inside     rune // shows whether it's inside a given collection, currently [ means it's an array
}

type tokenizer struct {
	s          string
	currentKey string
	i          int
}

func (t *tokenizer) readKey() (string, error) {
	start := t.i
	for ; t.i < len(t.s); t.i++ {
		if t.s[t.i] == '=' && t.i != len(t.s)-1 {
			t.i++

			return t.s[start : t.i-1], nil
		}
		if t.s[t.i] == ',' {
			k := t.s[start:t.i]

			return k, fmt.Errorf("key `%s` with no value", k)
		}
	}

	s := t.s[start:]

	return s, fmt.Errorf("key `%s` with no value", s)
}

func (t *tokenizer) readValue() string {
	start := t.i
	for ; t.i < len(t.s); t.i++ {
		if t.s[t.i] == ',' {
			t.i++

			return t.s[start : t.i-1]
		}
	}

	return t.s[start:]
}

func (t *tokenizer) readArray() (string, error) {
	start := t.i
	for ; t.i < len(t.s); t.i++ {
		if t.s[t.i] == ']' {
			if t.i+1 == len(t.s) || t.s[t.i+1] == ',' {
				t.i += 2

				return t.s[start : t.i-2], nil
			}
			t.i++

			return t.s[start : t.i-1], fmt.Errorf("there was no ',' after an array with key '%s'", t.currentKey)
		}
	}

	return t.s[start:], fmt.Errorf("array value for key `%s` didn't end", t.currentKey)
}

func tokenize(s string) ([]token, error) {
	result := []token{}
	t := &tokenizer{s: s}

	var err error
	var value string
	for t.i < len(s) {
		t.currentKey, err = t.readKey()
		if err != nil {
			return result, err
		}
		if t.s[t.i] == '[' {
			t.i++
			value, err = t.readArray()

			result = append(result, token{
				key:    t.currentKey,
				value:  value,
				inside: '[',
			})
			if err != nil {
				return result, err
			}
		} else {
			value = t.readValue()
			result = append(result, token{
				key:   t.currentKey,
				value: value,
			})
		}
	}

	return result, nil
}
