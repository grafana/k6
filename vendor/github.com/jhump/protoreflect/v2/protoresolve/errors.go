package protoresolve

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

var (
	// ErrNotFound is a sentinel error that is returned from resolvers to indicate that the named
	// element is not known to the registry. It is the same as protoregistry.NotFound.
	ErrNotFound = protoregistry.NotFound
)

// NewNotFoundError returns an error that wraps ErrNotFound with context
// indicating the given name as the element that could not be found.
//
// The parameter is generic so that it will accept both plain strings
// and named string types like protoreflect.FullName.
func NewNotFoundError[T ~string](name T) error {
	return fmt.Errorf("%s: %w", name, ErrNotFound)
}

// ErrUnexpectedType is an error that indicates a descriptor was resolved for
// a given URL or name, but it is of the wrong type. So a query may have been
// expecting a service descriptor, for example, but instead the queried name
// resolved to an extension descriptor.
//
// See NewUnexpectedTypeError.
type ErrUnexpectedType struct {
	// Only one of URL or Name will be set, depending on whether
	// the type was looked up by URL or name. These fields indicate
	// the query that resulted in a descriptor of the wrong type.
	URL  string
	Name protoreflect.FullName
	// The kind of descriptor that was expected.
	Expecting DescriptorKind
	// The kind of descriptor that was actually found.
	Actual DescriptorKind
	// Optional: the descriptor that was actually found. This may
	// be nil. If non-nil, this is the descriptor instance that
	// was resolved whose kind is Actual instead of Expecting.
	Descriptor protoreflect.Descriptor
}

// NewUnexpectedTypeError constructs a new *ErrUnexpectedType based
// on the given properties. The last parameter, url, is optional. If
// empty, the returned error will indicate that the given descriptor's
// full name as the query.
func NewUnexpectedTypeError(expecting DescriptorKind, got protoreflect.Descriptor, url string) *ErrUnexpectedType {
	var name protoreflect.FullName
	if url == "" {
		name = got.FullName()
	}
	return &ErrUnexpectedType{
		URL:        url,
		Name:       name,
		Expecting:  expecting,
		Descriptor: got,
	}
}

// Error implements the error interface.
func (e *ErrUnexpectedType) Error() string {
	var queryKind, query string
	if e.URL != "" {
		queryKind = "URL"
		query = e.URL
	} else {
		queryKind = "name"
		query = string(e.Name)
	}
	return fmt.Sprintf("wrong kind of descriptor for %s %q: expected %s, got %s", queryKind, query, e.Expecting.withArticle(), e.Actual.withArticle())
}
