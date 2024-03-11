package jwk

import (
	"github.com/lestrrat-go/option"
)

type identTypedField struct{}

type typedFieldPair struct {
	Name  string
	Value interface{}
}

// WithTypedField allows a private field to be parsed into the object type of
// your choice. It works much like the RegisterCustomField, but the effect
// is only applicable to the jwt.Parse function call which receives this option.
//
// While this can be extremely useful, this option should be used with caution:
// There are many caveats that your entire team/user-base needs to be aware of,
// and therefore in general its use is discouraged. Only use it when you know
// what you are doing, and you document its use clearly for others.
//
// First and foremost, this is a "per-object" option. Meaning that given the same
// serialized format, it is possible to generate two objects whose internal
// representations may differ. That is, if you parse one _WITH_ the option,
// and the other _WITHOUT_, their internal representation may completely differ.
// This could potentially lead to problems.
//
// Second, specifying this option will slightly slow down the decoding process
// as it needs to consult multiple definitions sources (global and local), so
// be careful if you are decoding a large number of tokens, as the effects will stack up.
func WithTypedField(name string, object interface{}) ParseOption {
	return &parseOption{
		option.New(identTypedField{},
			typedFieldPair{Name: name, Value: object},
		),
	}
}
