package k6deps

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strings"
)

//nolint:gochecknoglobals
var (
	// matches any sequence of characters enclosed by '/*' and '*/' including new lines and '*' not followed by '/'
	reMultiLineComment = regexp.MustCompile(`\/\*(?:[^\*]|\*[^\/]|\n)*(\*)+\/`)
	// matches '//' and any character until end of line skips the '//' sequence in urls (preceded by ':')
	reCommentLine = regexp.MustCompile(`(^|\s|[^:])(//.*)`)

	srcName       = `(?P<name>k6|k6/[^/]{2}.*|k6/[^x]/.*|k6/x/[/0-9a-zA-Z_-]+|(@[a-zA-Z0-9-_]+/)?xk6-([a-zA-Z0-9-_]+)((/[a-zA-Z0-9-_]+)*))` //nolint:lll
	srcConstraint = `=?v?0\.0\.0\+[0-9A-Za-z-]+|[vxX*|,&\^0-9.+-><=, ~]+`

	reName = regexp.MustCompile(srcName)

	srcModule  = strings.ReplaceAll(srcName, "name", "module")
	srcRequire = `require\("` + srcName + `"\)`
	srcImport  = `import (.* from )?["']` + srcModule + `["'](;|$)`

	reRequireOrImport        = regexp.MustCompile("(?m:" + srcRequire + "|" + srcImport + ")")
	idxRequireOrImportName   = reRequireOrImport.SubexpIndex("name")
	idxRequireOrImportModule = reRequireOrImport.SubexpIndex("module")

	reUseK6 = regexp.MustCompile(
		`"use +k6(( with ` + srcName + `( *(?P<constraints>` + srcConstraint + `))?)|(( *(?P<k6Constraints>` + srcConstraint + `)?)))"`) //nolint:lll

	idxUseName          = reUseK6.SubexpIndex("name")
	idxUseConstraints   = reUseK6.SubexpIndex("constraints")
	idxUseK6Constraints = reUseK6.SubexpIndex("k6Constraints")
)

// Dependencies contains the dependencies of the k6 test script in map format.
// The key of the map is the name of the dependency.
type Dependencies map[string]*Dependency

func (deps Dependencies) update(from *Dependency) error {
	dep, found := deps[from.Name]
	if !found {
		deps[from.Name] = from

		return nil
	}

	return dep.update(from)
}

// Merge updates deps dependencies based on from dependencies.
// Adds a dependency that doesn't exist yet.
// If the dependency exists in both collections, but one of them does not have version constraints,
// then the dependency with version constraints is placed in deps.
// Otherwise, i.e. if the dependency is included in both collections and in both with version constraints,
// an error is generated.
func (deps Dependencies) Merge(from Dependencies) error {
	for _, dep := range from {
		if err := deps.update(dep); err != nil {
			return err
		}
	}

	return nil
}

// Sorted returns dependencies as an array, with "k6" as a first element (if any) and the
// rest of the array is sorted by name lexicographically.
func (deps Dependencies) Sorted() []*Dependency {
	all := make([]*Dependency, 0, len(deps))

	for _, dep := range deps {
		all = append(all, dep)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].Name == NameK6 {
			return true
		}

		if all[j].Name == NameK6 {
			return false
		}

		return all[i].Name < all[j].Name
	})

	return all
}

// String converts the dependencies to displayable text format.
// The format is the same as that used by MarshalText.
func (deps Dependencies) String() string {
	text, _ := deps.MarshalText()

	return string(text)
}

// MarshalJSON marshals dependencies into JSON object format.
// The property names will be the dependency names, and the property
// values ​​will be the text format used by MarshalText of the given dependency.
func (deps Dependencies) MarshalJSON() ([]byte, error) {
	var buff bytes.Buffer

	encoder := json.NewEncoder(&buff)
	encoder.SetEscapeHTML(false)

	if _, err := buff.WriteRune('{'); err != nil {
		return nil, err
	}

	for idx, dep := range deps.Sorted() {
		if idx > 0 {
			buff.WriteRune(',')
		}

		if err := encoder.Encode(dep.Name); err != nil {
			return nil, err
		}

		buff.Truncate(buff.Len() - 1) // remove extra newline written by encoder

		if _, err := buff.WriteRune(':'); err != nil {
			return nil, err
		}

		if err := encoder.Encode(dep.GetConstraints().String()); err != nil {
			return nil, err
		}

		buff.Truncate(buff.Len() - 1) // remove extra newline written by encoder
	}

	if _, err := buff.WriteRune('}'); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// UnmarshalJSON unmarshals dependencies from a JSON object
// in the format used by MarshalJSON.
func (deps *Dependencies) UnmarshalJSON(data []byte) error {
	var unparsed map[string]string

	if err := json.Unmarshal(data, &unparsed); err != nil {
		return err
	}

	*deps = make(Dependencies, len(unparsed))

	for name, constraints := range unparsed {
		dep, err := NewDependency(name, constraints)
		if err != nil {
			return err
		}

		(*deps)[name] = dep
	}

	return nil
}

// MarshalText marshals the dependencies into a single-line text format.
// The text format is a semicolon-separated sequence of the text format of
// each dependency. The first element of the series is "k6" (if there is one),
// the following elements follow each other in lexically increasing order based on the name.
// For example: k6>0.49;k6/x/faker>=0.2.0;k6/x/toml>v0.1.0;xk6-dashboard*
func (deps Dependencies) MarshalText() ([]byte, error) {
	var buff bytes.Buffer

	if err := deps.marshalText(&buff); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

func (deps Dependencies) marshalText(w io.Writer) error {
	for idx, dep := range deps.Sorted() {
		if idx > 0 {
			if _, err := w.Write([]byte{';'}); err != nil {
				return err
			}
		}

		text, err := dep.MarshalText()
		if err != nil {
			return err
		}

		_, err = w.Write(text)
		if err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalText parses the one-line text dependencies format into the *deps variable.
func (deps *Dependencies) UnmarshalText(text []byte) error {
	*deps = make(Dependencies)

	start := bytes.LastIndexByte(text, ';') + 1
	end := len(text)

	for {
		var dep Dependency

		err := (&dep).UnmarshalText(text[start:end])
		if err != nil {
			return err
		}

		if err = deps.update(&dep); err != nil {
			return err
		}

		if start == 0 {
			break
		}

		end = start - 1
		start = bytes.LastIndexByte(text[:start-2], ';') + 1
	}

	return nil
}

// MarshalJS marshals dependencies into a consecutive, one-line JavaScript
// string directive format. The first element of the series is "k6" (if there is one),
// the following elements follow each other in lexically increasing order based on the name.
func (deps Dependencies) MarshalJS() ([]byte, error) {
	var buff bytes.Buffer

	if err := deps.marshalJS(&buff); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

func (deps Dependencies) marshalJS(w io.Writer) error {
	for _, dep := range deps.Sorted() {
		js, err := dep.MarshalJS()
		if err != nil {
			return err
		}

		_, err = w.Write(js)
		if err != nil {
			return err
		}

		_, err = w.Write([]byte{'\n'})
		if err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalJS unmarshals dependencies from a series of string directives
// in the format used by MarshalJS.
func (deps *Dependencies) UnmarshalJS(text []byte) error {
	*deps = make(Dependencies)

	// clean multiline comments
	clean := reMultiLineComment.ReplaceAll(text, []byte(""))

	// clean comment lines
	clean = reCommentLine.ReplaceAll(clean, []byte("$1"))

	for _, match := range reRequireOrImport.FindAllSubmatch(clean, -1) {
		extension := string(match[idxRequireOrImportName])
		if len(extension) == 0 {
			extension = string(match[idxRequireOrImportModule])
		}

		if len(extension) != 0 {
			// no negative lookahead regex support....
			if strings.HasPrefix(extension, "k6/") && !strings.HasPrefix(extension, "k6/x/") {
				extension = NameK6
			}

			_ = deps.update(&Dependency{Name: extension}) // no chance for conflicting
		}
	}

	return processUseDirectives(clean, *deps)
}

func processUseDirectives(text []byte, deps Dependencies) error {
	for _, match := range reUseK6.FindAllSubmatch(text, -1) {
		var dep *Dependency
		var err error

		if constraints := string(match[idxUseK6Constraints]); len(constraints) != 0 {
			dep, err = NewDependency(NameK6, constraints)
			if err != nil {
				return err
			}
		}

		if extension := string(match[idxUseName]); len(extension) != 0 {
			constraints := string(match[idxUseConstraints])

			dep, err = NewDependency(extension, constraints)
			if err != nil {
				return err
			}
		}

		if dep != nil {
			if err := deps.update(dep); err != nil {
				return err
			}
		}
	}

	return nil
}
