package k6deps

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/Masterminds/semver/v3"
)

const (
	// ConstraintsAny is a wildcard constraint that any version matches.
	ConstraintsAny = "*"

	// NameK6 is the name of the k6 dependency, its value is "k6".
	NameK6 = "k6"

	defaultConstraintsString = ConstraintsAny
)

//nolint:gochecknoglobals
var (
	ErrConstraints = errors.New("constraints error")
	ErrDependency  = errors.New("dependency error")

	defaultConstraints, _ = semver.NewConstraint(defaultConstraintsString)

	srcDependency = `(?P<name>[0-9a-zA-Z/@_-]+) *(?P<constraints>` + srcConstraint + `)?`

	reDependency             = regexp.MustCompile(srcDependency)
	idxDependencyName        = reDependency.SubexpIndex("name")
	idxDependencyConstraints = reDependency.SubexpIndex("constraints")
)

// Dependency contains the properties of a k6 dependency (extension or k6 core).
type Dependency struct {
	// Name is the name of the dependency.
	Name string `json:"name,omitempty"`
	// Constraints contains the version constraints of the dependency.
	Constraints *semver.Constraints `json:"constraints,omitempty"`
}

// NewDependency creates a new Dependency instance with the given name.
// If the constraints parameter is not empty, it will be parsed as version constraints.
func NewDependency(name, constraints string) (*Dependency, error) {
	var err error

	dep := new(Dependency)

	dep.Name = name

	if len(constraints) != 0 {
		if dep.Constraints, err = semver.NewConstraint(constraints); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrConstraints, err.Error())
		}
	}

	return dep, nil
}

// GetConstraints returns Constraints or the default constraints ("*") if Constraints is nil
func (dep *Dependency) GetConstraints() *semver.Constraints {
	if dep.Constraints == nil {
		return defaultConstraints
	}

	return dep.Constraints
}

// MarshalText marshals the dependency into a single-line text format.
// For example: k6/x/faker>0.1.0
func (dep *Dependency) MarshalText() ([]byte, error) {
	var buff bytes.Buffer

	if err := dep.marshalText(&buff); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

func (dep *Dependency) marshalText(w io.Writer) error {
	_, err := io.WriteString(w, dep.Name)
	if err != nil {
		return err
	}

	_, err = io.WriteString(w, dep.GetConstraints().String())
	return err
}

// MarshalJS marshals the dependency into a one-line JavaScript string directive format.
// For example: "us k6 with k6/x/faker>0.1.0";
func (dep *Dependency) MarshalJS() ([]byte, error) {
	var buff bytes.Buffer

	if err := dep.marshalJS(&buff); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

func (dep *Dependency) marshalJS(w io.Writer) error {
	_, err := io.WriteString(w, `"use k6`)
	if err != nil {
		return err
	}

	if dep.Name != NameK6 {
		_, err = io.WriteString(w, " with ")
		if err != nil {
			return err
		}

		_, err = io.WriteString(w, dep.Name)
		if err != nil {
			return err
		}
	}

	_, err = io.WriteString(w, dep.GetConstraints().String())
	if err != nil {
		return err
	}

	_, err = io.WriteString(w, `";`)
	return err
}

// UnmarshalText parses the one-line text dependency format into the *dep variable.
func (dep *Dependency) UnmarshalText(text []byte) error {
	match := reDependency.FindSubmatch(text)
	if match == nil {
		return fmt.Errorf("%w: invalid text format: %s", ErrDependency, string(text))
	}

	dep.Name = string(match[idxDependencyName])

	var err error

	dep.Constraints, err = semver.NewConstraint(string(match[idxDependencyConstraints]))
	if err != nil {
		return fmt.Errorf("%w: %s", ErrConstraints, err.Error())
	}

	return nil
}

// String converts the dependency to displayable text format.
// The format is the same as that used by MarshalText.
func (dep *Dependency) String() string {
	text, _ := dep.MarshalText()

	return string(text)
}

func (dep *Dependency) update(from *Dependency) error {
	fromString := from.GetConstraints().String()
	if fromString == defaultConstraintsString {
		return nil
	}

	depString := dep.GetConstraints().String()
	if depString == defaultConstraintsString {
		dep.Constraints = from.Constraints

		return nil
	}

	if depString == fromString {
		return nil
	}

	return fmt.Errorf("%w: %s has conflicting constraints:\n  %s\n  %s",
		ErrConstraints, dep.Name, dep.GetConstraints(), from.GetConstraints())
}
