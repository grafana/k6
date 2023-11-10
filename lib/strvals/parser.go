package strvals

import "fmt"

// Token represents a key-value token.
type Token struct {
	Key, Value string
	inside     rune // shows whether it's inside a given collection, currently [ means it's an array
}

type tokenizer struct {
	i          int
	s          string
	currentKey string
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

// Parse parses the input string into key-value tokens following strvals format:
// name=value,topname.subname=value.
func Parse(s string) ([]Token, error) {
	result := []Token{}
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

			result = append(result, Token{
				Key:    t.currentKey,
				Value:  value,
				inside: '[',
			})
			if err != nil {
				return result, err
			}
		} else {
			value = t.readValue()
			result = append(result, Token{
				Key:   t.currentKey,
				Value: value,
			})
		}
	}

	return result, nil
}
