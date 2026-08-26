[**xk6-kafka**](../README.md)

---

# Class: SchemaRegistry

Defined in: [index.d.ts:582](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L582)

## Classdesc

Schema Registry is a client for Schema Registry and handles serdes.

## Example

```javascript
// In init context
const writer = new Writer({
  brokers: ["localhost:9092"],
  topic: "my-topic",
  autoCreateTopic: true,
});

const schemaRegistry = new SchemaRegistry({
  url: "localhost:8081",
});

const keySchema = schemaRegistry.createSchema({
  version: 0,
  element: KEY,
  subject: "...",
  schema: "...",
  schemaType: "AVRO",
});

const valueSchema = schemaRegistry.createSchema({
  version: 0,
  element: VALUE,
  subject: "...",
  schema: "...",
  schemaType: "AVRO",
});

// In VU code (default function)
writer.produce({
  messages: [
    {
      key: schemaRegistry.serialize({
        data: "key",
        schema: keySchema,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
      value: schemaRegistry.serialize({
        data: "value",
        schema: valueSchema,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
    },
  ],
});
```

## Constructors

### Constructor

> **new SchemaRegistry**(`schemaRegistryConfig`): `SchemaRegistry`

Defined in: [index.d.ts:589](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L589)

#### Parameters

##### schemaRegistryConfig

[`SchemaRegistryConfig`](../interfaces/SchemaRegistryConfig.md)

Schema Registry configuration.

#### Returns

`SchemaRegistry`

- SchemaRegistry instance.

## Methods

### createSchema()

> **createSchema**(`schema`): [`Schema`](../interfaces/Schema.md)

Defined in: [index.d.ts:603](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L603)

#### Parameters

##### schema

[`Schema`](../interfaces/Schema.md)

Schema configuration.

#### Returns

[`Schema`](../interfaces/Schema.md)

- Schema.

#### Method

Create or update a schema on Schema Registry.

---

### deserialize()

> **deserialize**(`container`): `any`

Defined in: [index.d.ts:624](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L624)

#### Parameters

##### container

[`Container`](../interfaces/Container.md)

Container including data, schema and schemaType.

#### Returns

`any`

- Deserialized data as string, byte array or JSON object.

#### Method

Deserializes the given data and schema into its original form.

---

### getSchema()

> **getSchema**(`schema`): [`Schema`](../interfaces/Schema.md)

Defined in: [index.d.ts:596](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L596)

#### Parameters

##### schema

[`Schema`](../interfaces/Schema.md)

Schema configuration.

#### Returns

[`Schema`](../interfaces/Schema.md)

- Schema.

#### Method

Get a schema from Schema Registry by version and subject.

---

### getSubjectName()

> **getSubjectName**(`subjectNameConfig`): `string`

Defined in: [index.d.ts:610](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L610)

#### Parameters

##### subjectNameConfig

[`SubjectNameConfig`](../interfaces/SubjectNameConfig.md)

Subject name configuration.

#### Returns

`string`

- Subject name.

#### Method

Returns the subject name for the given SubjectNameConfig.

---

### serialize()

> **serialize**(`container`): `Uint8Array`

Defined in: [index.d.ts:617](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L617)

#### Parameters

##### container

[`Container`](../interfaces/Container.md)

Container including data, schema and schemaType.

#### Returns

`Uint8Array`

- Serialized data as byte array.

#### Method

Serializes the given data and schema into a byte array.
