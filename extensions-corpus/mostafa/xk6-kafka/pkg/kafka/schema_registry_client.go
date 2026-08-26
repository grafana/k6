package kafka

import cschemaregistry "github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"

type SchemaRegistryClient interface {
	GetLatestSchema(subject string) (*RegisteredSchema, error)
	GetSchemaByVersion(subject string, version int) (*RegisteredSchema, error)
	CreateSchema(
		subject string,
		schema string,
		schemaType SchemaType,
		references ...Reference,
	) (*RegisteredSchema, error)
	Close() error
}

type confluentSchemaRegistryAdapter struct {
	cacheEnabled bool
	client       cschemaregistry.Client
}

func newConfluentSchemaRegistryAdapter(
	client cschemaregistry.Client,
	cacheEnabled bool,
) SchemaRegistryClient {
	if client == nil {
		return nil
	}

	return &confluentSchemaRegistryAdapter{
		cacheEnabled: cacheEnabled,
		client:       client,
	}
}

func (a *confluentSchemaRegistryAdapter) GetLatestSchema(subject string) (*RegisteredSchema, error) {
	defer a.clearCachesIfDisabled()

	metadata, err := a.client.GetLatestSchemaMetadata(subject)
	if err != nil {
		return nil, err
	}

	return newRegisteredSchema(
		metadata.ID,
		metadata.Version,
		metadata.Schema,
		metadata.SchemaType,
		metadata.References,
	), nil
}

func (a *confluentSchemaRegistryAdapter) GetSchemaByVersion(subject string, version int) (*RegisteredSchema, error) {
	defer a.clearCachesIfDisabled()

	metadata, err := a.client.GetSchemaMetadata(subject, version)
	if err != nil {
		return nil, err
	}

	return newRegisteredSchema(
		metadata.ID,
		metadata.Version,
		metadata.Schema,
		metadata.SchemaType,
		metadata.References,
	), nil
}

func (a *confluentSchemaRegistryAdapter) CreateSchema(
	subject string,
	schema string,
	schemaType SchemaType,
	references ...Reference,
) (*RegisteredSchema, error) {
	defer a.clearCachesIfDisabled()

	schemaInfo := newConfluentSchemaInfo(schema, schemaType, references)

	metadata, err := a.client.RegisterFullResponse(subject, schemaInfo, false)
	if err != nil {
		return nil, err
	}

	metadata = a.hydrateRegisteredMetadata(subject, schemaInfo, metadata)

	return newRegisteredSchema(
		metadata.ID,
		metadata.Version,
		metadata.Schema,
		metadata.SchemaType,
		metadata.References,
	), nil
}

func (a *confluentSchemaRegistryAdapter) Close() error {
	return a.client.Close()
}

func (a *confluentSchemaRegistryAdapter) clearCachesIfDisabled() {
	if a.cacheEnabled || a.client == nil {
		return
	}

	_ = a.client.ClearCaches()
}

func newConfluentSchemaInfo(
	schema string,
	schemaType SchemaType,
	references []Reference,
) cschemaregistry.SchemaInfo {
	return cschemaregistry.SchemaInfo{
		Schema:     schema,
		SchemaType: string(normalizeSchemaType(string(schemaType))),
		References: references,
	}
}

func (a *confluentSchemaRegistryAdapter) hydrateRegisteredMetadata(
	subject string,
	schemaInfo cschemaregistry.SchemaInfo,
	metadata cschemaregistry.SchemaMetadata,
) cschemaregistry.SchemaMetadata {
	if metadata.Subject == "" {
		metadata.Subject = subject
	}
	if metadata.Schema == "" {
		metadata.Schema = schemaInfo.Schema
	}
	if metadata.SchemaType == "" {
		metadata.SchemaType = schemaInfo.SchemaType
	}
	if len(metadata.References) == 0 && len(schemaInfo.References) > 0 {
		metadata.References = schemaInfo.References
	}
	if metadata.Version > 0 {
		return metadata
	}

	version, err := a.client.GetVersion(subject, schemaInfo, false)
	if err != nil || version <= 0 {
		return metadata
	}

	metadata.Version = version

	hydrated, err := a.client.GetSchemaMetadata(subject, version)
	if err != nil {
		return metadata
	}

	if hydrated.Subject == "" {
		hydrated.Subject = subject
	}
	if hydrated.ID == 0 {
		hydrated.ID = metadata.ID
	}
	if hydrated.Schema == "" {
		hydrated.Schema = schemaInfo.Schema
	}
	if hydrated.SchemaType == "" {
		hydrated.SchemaType = schemaInfo.SchemaType
	}
	if hydrated.Version == 0 {
		hydrated.Version = version
	}
	if len(hydrated.References) == 0 && len(schemaInfo.References) > 0 {
		hydrated.References = schemaInfo.References
	}

	return hydrated
}
