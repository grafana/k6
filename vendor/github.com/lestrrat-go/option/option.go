package option

import "fmt"

// Interface defines the minimum interface that an option must fulfill
type Interface interface {
	// Ident returns the "identity" of this option, a unique identifier that
	// can be used to differentiate between options
	Ident() interface{}

	// Value returns the corresponding value.
	Value() interface{}
}

type pair struct {
	ident interface{}
	value interface{}
}

// New creates a new Option
func New(ident, value interface{}) Interface {
	return &pair{
		ident: ident,
		value: value,
	}
}

func (p *pair) Ident() interface{} {
	return p.ident
}

func (p *pair) Value() interface{} {
	return p.value
}

func (p *pair) String() string {
	return fmt.Sprintf(`%v(%v)`, p.ident, p.value)
}
