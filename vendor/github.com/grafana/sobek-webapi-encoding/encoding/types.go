package encoding

// Name is the identifier of an encoding format as defined by the WHATWG Encoding spec.
// See https://encoding.spec.whatwg.org/#names-and-labels
type Name string

const (
	// UTF8EncodingFormat is the encoding format for UTF-8.
	UTF8EncodingFormat Name = "utf-8"

	// UTF16LEEncodingFormat is the encoding format for UTF-16LE (little-endian).
	UTF16LEEncodingFormat Name = "utf-16le"

	// UTF16BEEncodingFormat is the encoding format for UTF-16BE (big-endian).
	UTF16BEEncodingFormat Name = "utf-16be"
)
