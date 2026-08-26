package kafka

import cschemaregistry "github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"

type SchemaType string

const (
	Protobuf SchemaType = "PROTOBUF"
	Avro     SchemaType = "AVRO"
	Json     SchemaType = "JSON"
)

type Reference = cschemaregistry.Reference

type RegisteredSchema struct {
	id         int
	schema     string
	schemaType *SchemaType
	version    int
	references []Reference
}

func newRegisteredSchema(
	id int,
	version int,
	schema string,
	schemaType string,
	references []Reference,
) *RegisteredSchema {
	normalizedType := normalizeSchemaType(schemaType)

	return &RegisteredSchema{
		id:         id,
		schema:     schema,
		schemaType: &normalizedType,
		version:    version,
		references: references,
	}
}

func normalizeSchemaType(schemaType string) SchemaType {
	if schemaType == "" {
		return Avro
	}

	return SchemaType(schemaType)
}

func (s *RegisteredSchema) ID() int {
	return s.id
}

func (s *RegisteredSchema) Schema() string {
	return s.schema
}

func (s *RegisteredSchema) SchemaType() *SchemaType {
	return s.schemaType
}

func (s *RegisteredSchema) Version() int {
	return s.version
}

func (s *RegisteredSchema) References() []Reference {
	return s.references
}
