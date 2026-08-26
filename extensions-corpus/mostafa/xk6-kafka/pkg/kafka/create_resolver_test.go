package kafka

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errStubNoLatestMap       = errors.New("stub: no latest map")
	errStubUnknownLatest     = errors.New("stub: unknown latest subject")
	errStubNoByVersionMap    = errors.New("stub: no byVersion map")
	errStubUnknownSchemaVer  = errors.New("stub: unknown schema version")
	errStubCreateSchemaUnsup = errors.New("stub: CreateSchema not implemented")
)

// stubSchemaRegistryClient implements SchemaRegistryClient for resolver tests.
type stubSchemaRegistryClient struct {
	latestBySubject map[string]*RegisteredSchema
	byVersion       map[string]map[int]*RegisteredSchema
}

func (s *stubSchemaRegistryClient) GetLatestSchema(subject string) (*RegisteredSchema, error) {
	if s.latestBySubject == nil {
		return nil, errStubNoLatestMap
	}
	if r, ok := s.latestBySubject[subject]; ok {
		return r, nil
	}
	_ = subject
	return nil, errStubUnknownLatest
}

func (s *stubSchemaRegistryClient) GetSchemaByVersion(subject string, version int) (*RegisteredSchema, error) {
	if s.byVersion == nil {
		return nil, errStubNoByVersionMap
	}
	if m, ok := s.byVersion[subject]; ok {
		if r, ok2 := m[version]; ok2 {
			return r, nil
		}
	}
	_ = subject
	_ = version
	return nil, errStubUnknownSchemaVer
}

func (s *stubSchemaRegistryClient) CreateSchema(
	_ string,
	_ string,
	_ SchemaType,
	_ ...Reference,
) (*RegisteredSchema, error) {
	return nil, errStubCreateSchemaUnsup
}

func (s *stubSchemaRegistryClient) Close() error {
	return nil
}

func TestCreateResolverWithCacheFetchesLatest(t *testing.T) {
	test := getTestModuleInstance(t)
	k := test.module

	reg := newRegisteredSchema(7, 3, `{"type":"record","name":"R","fields":[]}`, string(Avro), nil)
	stub := &stubSchemaRegistryClient{
		latestBySubject: map[string]*RegisteredSchema{
			"resolved-subject": reg,
		},
	}
	cache := map[string]*Schema{}
	resolver := k.createResolverWithCache(stub, cache, true)

	got, err := resolver("resolved-subject")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 7, got.ID)
	assert.Equal(t, 3, got.Version)
	assert.Equal(t, "resolved-subject", got.Subject)
	assert.Contains(t, cache, "resolved-subject")
}

func TestCreateResolverWithCacheHitsCacheBySubject(t *testing.T) {
	test := getTestModuleInstance(t)
	k := test.module

	avroT := Avro
	cached := &Schema{
		ID:            1,
		Schema:        `{"type":"record","name":"Cached","fields":[]}`,
		SchemaType:    &avroT,
		Version:       1,
		Subject:       "cached-key",
		EnableCaching: true,
	}
	cache := map[string]*Schema{"cached-key": cached}
	stub := &stubSchemaRegistryClient{} // GetLatest not used

	resolver := k.createResolverWithCache(stub, cache, true)
	got, err := resolver("cached-key")
	require.NoError(t, err)
	assert.Equal(t, cached, got)
}

func TestCreateResolverWithCacheUsesReferenceGetSchemaByVersion(t *testing.T) {
	test := getTestModuleInstance(t)
	k := test.module

	avroT := Avro
	childReg := newRegisteredSchema(9, 1, `{"type":"record","name":"Child","fields":[]}`, string(Avro), nil)

	parent := &Schema{
		ID:            2,
		Schema:        `{"type":"record","name":"Parent","fields":[]}`,
		SchemaType:    &avroT,
		Version:       1,
		Subject:       "parent-subj",
		EnableCaching: true,
		References: []Reference{{
			Name:    "com.example.Child",
			Subject: "child-registry-subj",
			Version: 1,
		}},
	}
	cache := map[string]*Schema{"parent-subj": parent}

	stub := &stubSchemaRegistryClient{
		latestBySubject: map[string]*RegisteredSchema{}, // empty so GetLatest fails for child name
		byVersion: map[string]map[int]*RegisteredSchema{
			"child-registry-subj": {1: childReg},
		},
	}

	resolver := k.createResolverWithCache(stub, cache, true)
	got, err := resolver("com.example.Child")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 9, got.ID)
	assert.Equal(t, "child-registry-subj", got.Subject)
}

func TestCreateResolverWithCacheMatchesFullNameFromJSON(t *testing.T) {
	test := getTestModuleInstance(t)
	k := test.module

	avroT := Avro
	schemaJSON := `{"type":"record","name":"X","namespace":"com.ns","fields":[]}`
	cached := &Schema{
		ID:            3,
		Schema:        schemaJSON,
		SchemaType:    &avroT,
		Version:       1,
		Subject:       "other-subject",
		EnableCaching: true,
		avroSchema:    nil,
	}
	cache := map[string]*Schema{"other-subject": cached}
	stub := &stubSchemaRegistryClient{}

	resolver := k.createResolverWithCache(stub, cache, true)
	got, err := resolver("com.ns.X")
	require.NoError(t, err)
	assert.Equal(t, cached, got)
}

func TestCreateResolverWithCacheFetchWithoutCachingInMap(t *testing.T) {
	test := getTestModuleInstance(t)
	k := test.module

	reg := newRegisteredSchema(11, 1, `{"type":"record","name":"S","fields":[]}`, string(Avro), nil)
	stub := &stubSchemaRegistryClient{
		latestBySubject: map[string]*RegisteredSchema{"live": reg},
	}
	cache := map[string]*Schema{}

	resolver := k.createResolverWithCache(stub, cache, false)
	got, err := resolver("live")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 11, got.ID)
	_, has := cache["live"]
	assert.False(t, has)
}

func TestCreateResolverWithCacheNotFound(t *testing.T) {
	test := getTestModuleInstance(t)
	k := test.module

	stub := &stubSchemaRegistryClient{latestBySubject: map[string]*RegisteredSchema{}}
	cache := map[string]*Schema{}

	resolver := k.createResolverWithCache(stub, cache, true)
	got, err := resolver("missing.everywhere")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrReferenceNotFound)
}
