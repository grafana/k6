# Schema Registry usage with xk6-kafka

## What is Schema Registry?

Schema Registry is a service that provides a centralized repository for managing and validating schemas used in data serialization formats like Avro, JSON, and Protobuf.
It ensures that the data produced and consumed by Kafka producers and consumers adheres to a defined schema, enabling compatibility and evolution of data structures over time.
It is highly recommended to use Schema Registry when working with Kafka, in order to ensure that your messages conform to the expected structure and types.

> [!NOTE]
> `xk6-kafka v2.1.0` supports Avro, JSON, and Protobuf on both Schema Registry and standalone schema flows.

## How to Use Schema Registry with xk6-kafka

### Create a Schema Registry Client

To use Schema Registry with xk6-kafka,
you need to create a Schema Registry client that will handle the communication with the Schema Registry service.

#### No authentication

```javascript
const schemaRegistry = new SchemaRegistry({
  url: "http://localhost:8081",
});
```

#### Basic authentication

```javascript
const schemaRegistry = new SchemaRegistry({
  url: "http://localhost:8081",
  basicAuth: {
    username: "my-username",
    password: "my-password",
  },
});
```

### Create a new Schema into your Schema Registry

If you have not already created a schema in your Schema Registry, you can do so using the following steps
to let your load test create it for you.

```javascript
const keySchema = `{
  "name": "KeySchema",
  "type": "record",
  "namespace": "com.example.key",
  "fields": [
    {
      "name": "ssn",
      "type": "string"
    }
  ]
}
`;
const valueSchema = `{
  "name": "ValueSchema",
  "type": "record",
  "namespace": "com.example.value",
  "fields": [
    {
      "name": "firstName",
      "type": "string"
    },
    {
      "name": "lastName",
      "type": "string"
    }
  ]
}`;

const keySchemaObject = schemaRegistry.createSchema({
  subject: "my-key-schema-name",
  schema: keySchema,
  schemaType: SCHEMA_TYPE_AVRO,
});

const valueSchemaObject = schemaRegistry.createSchema({
  subject: "my-value-schema-name",
  schema: valueSchema,
  schemaType: SCHEMA_TYPE_AVRO,
});
```

Optional : Retrieve the subject name directly from the schema:

```javascript
const keySubjectName = schemaRegistry.getSubjectName({
  topic: topic,
  element: KEY,
  subjectNameStrategy: TOPIC_NAME_STRATEGY, // or RECORD_NAME_STRATEGY depending on your needs
  schema: keySchema,
});

const valueSubjectName = schemaRegistry.getSubjectName({
  topic: topic,
  element: VALUE,
  subjectNameStrategy: RECORD_NAME_STRATEGY, // or TOPIC_NAME_STRATEGY depending on your needs
  schema: valueSchema,
});
```

You can then use these schemas in your load test to produce and consume messages.

```javascript
const messages = [
  {
    key: schemaRegistry.serialize({
      data: {
        ssn: "123-45-6789",
      },
      schema: keySchemaObject,
      schemaType: SCHEMA_TYPE_AVRO,
    }),
    value: schemaRegistry.serialize({
      data: {
        firstName: "John",
        lastName: "Doe",
      },
      schema: valueSchemaObject,
      schemaType: SCHEMA_TYPE_AVRO,
    }),
    headers: {
      mykey: "myvalue",
    },
    offset: 0,
    partition: 0,
    time: new Date(),
  },
];
```

### Load an existing Schema from your Schema Registry

If you already have a schema registered in your Schema Registry, you can load it using the following method:

```javascript
const keySchemaObject = schemaRegistry.getSchema({
  subject: "my-key-schema-name",
});

const valueSchemaObject = schemaRegistry.getSchema({
  subject: "my-value-schema-name",
});

const messages = [
  {
    key: schemaRegistry.serialize({
      data: {
        ssn: "123-45-6789",
      },
      schema: keySchemaObject,
      schemaType: SCHEMA_TYPE_AVRO,
    }),
    value: schemaRegistry.serialize({
      data: {
        firstName: "John",
        lastName: "Doe",
      },
      schema: valueSchemaObject,
      schemaType: SCHEMA_TYPE_AVRO,
    }),
    headers: {
      mykey: "myvalue",
    },
    offset: 0,
    partition: 0,
    time: new Date(),
  },
];
```

### Complex schemas : Manage union types

When dealing with complex schemas, especially those involving union types, you'll have to ensure that the data you serialize matches the expected schema structure.
Union types in Avro allow a field to hold values of different types, which can complicate serialization and deserialization.
For example, if you have a union type schema like this:

```json
{
  "name": "MyUnionSchema",
  "type": "record",
  "fields": [
    {
      "name": "myField",
      "type": ["null", "string"]
    }
  ]
}
```

Here `myField` can either be a string or null. When serializing data for this schema with hamba/avro, you can provide the value directly:

```javascript
export const productionOrder = {
  myField: "My awesome value", // For string value
  // or
  // myField: null, // For null value
};
```

With hamba/avro, you usually don't need to wrap union values in a type-specific object. Simply provide the value that matches one of the union types. For nullable fields, you can use `null` directly.

For unions with logical primitive types (for example `["null", {"type":"int","logicalType":"date"}]`), xk6-kafka accepts:

- direct values (for example `documentValidTo: 20474`)
- wrapped primitive values (for example `documentValidTo: { "int": 20474 }`)
- wrapped logical discriminator values (for example `documentValidTo: { "int.date": 20474 }`)

xk6-kafka normalizes these inputs before Avro encoding so they match the discriminator format expected by `hamba/avro`.

**Note**: In your schema, you should define the `null` value first in the union type (e.g., `["null", "string"]` rather than `["string", "null"]`) to follow Avro best practices, though hamba/avro will handle both cases.

In order to help you with complex schemas, you can use the [nested-avro-schema](https://github.com/mostafa/nested-avro-schema) project, which provides a way to define complex Avro schemas in a more structured way.

---

## 🔧 Complete Example with Topic Management

Here's a complete example showing schema registry usage with proper topic lifecycle management:

```javascript
import { check, sleep } from "k6";
import {
  Writer,
  Reader,
  Connection,
  SchemaRegistry,
  SCHEMA_TYPE_AVRO,
} from "k6/x/kafka";

const brokers = ["localhost:9092"];
const topic = "my-avro-topic";

// Create Writer and Reader at module level
const writer = new Writer({
  brokers: brokers,
  topic: topic,
});

const reader = new Reader({
  brokers: brokers,
  topic: topic,
});

// Create Schema Registry client
const schemaRegistry = new SchemaRegistry({
  url: "http://localhost:8081",
});

// Define your schema
const valueSchema = `{
  "type": "record",
  "name": "User",
  "namespace": "com.example",
  "fields": [
    { "name": "firstName", "type": "string" },
    { "name": "lastName", "type": "string" },
    { "name": "age", "type": "int" }
  ]
}`;

// Get schema object from registry
const valueSchemaObject = schemaRegistry.getSchema({
  subject: "my-avro-topic-value",
});

export function setup() {
  // Create topic in setup() to avoid race conditions
  const connection = new Connection({
    address: brokers[0],
  });

  connection.createTopic({
    topic: topic,
    numPartitions: 3,
  });

  // Verify topic was created
  const topics = connection.listTopics();
  if (!topics.includes(topic)) {
    throw new Error(`Topic ${topic} was not created successfully`);
  }

  connection.close();

  // Wait for Kafka metadata to propagate to all brokers
  sleep(2);
}

export default function () {
  // Produce Avro message
  const messages = [
    {
      value: schemaRegistry.serialize({
        data: {
          firstName: "John",
          lastName: "Doe",
          age: 30,
        },
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
    },
  ];

  writer.produce({ messages: messages });

  // Consume and deserialize
  const consumed = reader.consume({ limit: 1 });

  if (consumed.length > 0) {
    const user = schemaRegistry.deserialize({
      data: consumed[0].value,
      schema: valueSchemaObject,
      schemaType: SCHEMA_TYPE_AVRO,
    });

    check(user, {
      "firstName is correct": (u) => u.firstName === "John",
      "age is correct": (u) => u.age === 30,
    });
  }
}

export function teardown() {
  // Clean up
  const connection = new Connection({
    address: brokers[0],
  });
  connection.deleteTopic(topic);
  connection.close();
  writer.close();
  reader.close();
}
```

**Key Points:**

- Topic creation in `setup()` ensures all VUs wait before producing
- Connection is created inside `setup()` for proper VU context
- `sleep(2)` allows Kafka metadata to propagate
- Schema objects are created at module level (one-time operation)
- Serialization/deserialization happens in the VU function

For more details on topic management, see the [Writers documentation](./writers.md#topic-management).
