package jwk

import (
	"context"
	"sync"

	"github.com/lestrrat-go/iter/arrayiter"
	"github.com/lestrrat-go/iter/mapiter"
	"github.com/lestrrat-go/jwx/v2/internal/iter"
	"github.com/lestrrat-go/jwx/v2/internal/json"
)

// AsymmetricKey describes a Key that represents an key in an asymmetric key pair,
// which in turn can be either a private or a public key. This interface
// allows those keys to be queried if they are one or the other.
type AsymmetricKey interface {
	IsPrivate() bool
}

// KeyUsageType is used to denote what this key should be used for
type KeyUsageType string

const (
	// ForSignature is the value used in the headers to indicate that
	// this key should be used for signatures
	ForSignature KeyUsageType = "sig"
	// ForEncryption is the value used in the headers to indicate that
	// this key should be used for encrypting
	ForEncryption KeyUsageType = "enc"
)

type KeyOperation string
type KeyOperationList []KeyOperation

const (
	KeyOpSign       KeyOperation = "sign"       // (compute digital signature or MAC)
	KeyOpVerify     KeyOperation = "verify"     // (verify digital signature or MAC)
	KeyOpEncrypt    KeyOperation = "encrypt"    // (encrypt content)
	KeyOpDecrypt    KeyOperation = "decrypt"    // (decrypt content and validate decryption, if applicable)
	KeyOpWrapKey    KeyOperation = "wrapKey"    // (encrypt key)
	KeyOpUnwrapKey  KeyOperation = "unwrapKey"  // (decrypt key and validate decryption, if applicable)
	KeyOpDeriveKey  KeyOperation = "deriveKey"  // (derive key)
	KeyOpDeriveBits KeyOperation = "deriveBits" // (derive bits not to be used as a key)
)

// Set represents JWKS object, a collection of jwk.Key objects.
//
// Sets can be safely converted to and from JSON using the standard
// `"encoding/json".Marshal` and `"encoding/json".Unmarshal`. However,
// if you do not know if the payload contains a single JWK or a JWK set,
// consider using `jwk.Parse()` to always get a `jwk.Set` out of it.
//
// Since v1.2.12, JWK sets with private parameters can be parsed as well.
// Such private parameters can be accessed via the `Field()` method.
// If a resource contains a single JWK instead of a JWK set, private parameters
// are stored in _both_ the resulting `jwk.Set` object and the `jwk.Key` object .
//
//nolint:interfacebloat
type Set interface {
	// AddKey adds the specified key. If the key already exists in the set,
	// an error is returned.
	AddKey(Key) error

	// Clear resets the list of keys associated with this set, emptying the
	// internal list of `jwk.Key`s, as well as clearing any other non-key
	// fields
	Clear() error

	// Get returns the key at index `idx`. If the index is out of range,
	// then the second return value is false.
	Key(int) (Key, bool)

	// Get returns the value of a private field in the key set.
	//
	// For the purposes of a key set, any field other than the "keys" field is
	// considered to be a private field. In other words, you cannot use this
	// method to directly access the list of keys in the set
	Get(string) (interface{}, bool)

	// Set sets the value of a single field.
	//
	// This method, which takes an `interface{}`, exists because
	// these objects can contain extra _arbitrary_ fields that users can
	// specify, and there is no way of knowing what type they could be.
	Set(string, interface{}) error

	// Remove removes the specified non-key field from the set.
	// Keys may not be removed using this method. See RemoveKey for
	// removing keys.
	Remove(string) error

	// Index returns the index where the given key exists, -1 otherwise
	Index(Key) int

	// Len returns the number of keys in the set
	Len() int

	// LookupKeyID returns the first key matching the given key id.
	// The second return value is false if there are no keys matching the key id.
	// The set *may* contain multiple keys with the same key id. If you
	// need all of them, use `Iterate()`
	LookupKeyID(string) (Key, bool)

	// RemoveKey removes the key from the set.
	// RemoveKey returns an error when the specified key does not exist
	// in set.
	RemoveKey(Key) error

	// Keys creates an iterator to iterate through all keys in the set.
	Keys(context.Context) KeyIterator

	// Iterate creates an iterator to iterate through all fields other than the keys
	Iterate(context.Context) HeaderIterator

	// Clone create a new set with identical keys. Keys themselves are not cloned.
	Clone() (Set, error)
}

type set struct {
	keys          []Key
	mu            sync.RWMutex
	dc            DecodeCtx
	privateParams map[string]interface{}
}

type HeaderVisitor = iter.MapVisitor
type HeaderVisitorFunc = iter.MapVisitorFunc
type HeaderPair = mapiter.Pair
type HeaderIterator = mapiter.Iterator
type KeyPair = arrayiter.Pair
type KeyIterator = arrayiter.Iterator

type PublicKeyer interface {
	// PublicKey creates the corresponding PublicKey type for this object.
	// All fields are copied onto the new public key, except for those that are not allowed.
	// Returned value must not be the receiver itself.
	PublicKey() (Key, error)
}

type DecodeCtx interface {
	json.DecodeCtx
	IgnoreParseError() bool
}
type KeyWithDecodeCtx interface {
	SetDecodeCtx(DecodeCtx)
	DecodeCtx() DecodeCtx
}
