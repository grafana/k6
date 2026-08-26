/*
 * This is a comprehensive k6 test script that tests Kafka with complex Avro schemas
 * including nested records, unions, arrays, maps, enums, fixed types, and combinations thereof.
 * This script demonstrates the full capabilities of the hamba/avro library.
 *
 * NESTED SCHEMA REFERENCES:
 * This script uses Schema Registry references to demonstrate nested schemas. Instead of
 * defining all types inline, it registers separate schemas for nested types (enums, records)
 * and then references them in the main schema. This approach:
 * - Allows schema reuse across multiple schemas
 * - Enables independent versioning of nested types
 * - Demonstrates Schema Registry's reference capabilities
 *
 * The setup() function registers the following nested schemas:
 * - UserStatus (enum)
 * - Currency (enum)
 * - PaymentMethod (enum)
 * - UserPreferences (record)
 * - User (record, references UserStatus and UserPreferences)
 * - OrderItem (record)
 * - Order (record, references Currency, PaymentMethod, and OrderItem)
 * - Coordinates (record)
 *
 * The main ComplexValue schema then references User, Order, and Coordinates.
 */

import { check, sleep } from "k6";
import {
  Writer,
  Reader,
  Connection,
  SchemaRegistry,
  KEY,
  VALUE,
  TOPIC_NAME_STRATEGY,
  RECORD_NAME_STRATEGY,
  SCHEMA_TYPE_AVRO,
} from "k6/x/kafka";

const brokers = ["localhost:9092"];
const topic = "com.example.complex_avro";

const writer = new Writer({
  brokers: brokers,
  topic: topic,
});

const reader = new Reader({
  brokers: brokers,
  topic: topic,
});

const schemaRegistry = new SchemaRegistry({
  url: "http://localhost:8081",
});

export function setup() {
  const connection = new Connection({
    address: brokers[0],
  });

  connection.createTopic({ topic: topic });

  // Verify topic was created
  const topics = connection.listTopics();
  if (!topics.includes(topic)) {
    throw new Error(`Topic ${topic} was not created successfully`);
  }

  connection.close();

  // Wait for Kafka metadata to propagate to all brokers
  sleep(2);

  // Register nested schemas first (these will be referenced by the main schema)
  // These schemas are registered as separate subjects in Schema Registry

  // 1. Register UserStatus enum
  const userStatusSchema = `{
    "name": "UserStatus",
    "type": "enum",
    "namespace": "com.example.complex",
    "symbols": ["ACTIVE", "INACTIVE", "SUSPENDED", "PENDING"]
  }`;

  const userStatusSubject = "com.example.complex.UserStatus";
  const userStatusSchemaObject = schemaRegistry.createSchema({
    subject: userStatusSubject,
    schema: userStatusSchema,
    schemaType: SCHEMA_TYPE_AVRO,
  });

  // 2. Register Currency enum
  const currencySchema = `{
    "name": "Currency",
    "type": "enum",
    "namespace": "com.example.complex",
    "symbols": ["USD", "EUR", "GBP", "JPY"]
  }`;

  const currencySubject = "com.example.complex.Currency";
  const currencySchemaObject = schemaRegistry.createSchema({
    subject: currencySubject,
    schema: currencySchema,
    schemaType: SCHEMA_TYPE_AVRO,
  });

  // 3. Register PaymentMethod enum
  const paymentMethodSchema = `{
    "name": "PaymentMethod",
    "type": "enum",
    "namespace": "com.example.complex",
    "symbols": ["CREDIT_CARD", "DEBIT_CARD", "PAYPAL", "BANK_TRANSFER"]
  }`;

  const paymentMethodSubject = "com.example.complex.PaymentMethod";
  const paymentMethodSchemaObject = schemaRegistry.createSchema({
    subject: paymentMethodSubject,
    schema: paymentMethodSchema,
    schemaType: SCHEMA_TYPE_AVRO,
  });

  // 4. Register SHA256 fixed type (for testing fixed type serialization)
  const sha256Schema = `{
    "name": "SHA256",
    "type": "fixed",
    "namespace": "com.example.complex",
    "size": 32
  }`;

  const sha256Subject = "com.example.complex.SHA256";
  const sha256SchemaObject = schemaRegistry.createSchema({
    subject: sha256Subject,
    schema: sha256Schema,
    schemaType: SCHEMA_TYPE_AVRO,
  });

  // 5. Register UserPreferences record
  const userPreferencesSchema = `{
    "name": "UserPreferences",
    "type": "record",
    "namespace": "com.example.complex",
    "fields": [
      {
        "name": "theme",
        "type": ["null", "string"],
        "default": null
      },
      {
        "name": "notifications",
        "type": "boolean",
        "default": true
      },
      {
        "name": "tags",
        "type": {
          "type": "array",
          "items": "string"
        }
      }
    ]
  }`;

  const userPreferencesSubject = "com.example.complex.UserPreferences";
  const userPreferencesSchemaObject = schemaRegistry.createSchema({
    subject: userPreferencesSubject,
    schema: userPreferencesSchema,
    schemaType: SCHEMA_TYPE_AVRO,
  });

  // 5. Register User record (references UserStatus and UserPreferences)
  const userSchema = `{
    "name": "User",
    "type": "record",
    "namespace": "com.example.complex",
    "fields": [
      {
        "name": "id",
        "type": "long"
      },
      {
        "name": "username",
        "type": "string"
      },
      {
        "name": "email",
        "type": ["null", "string"],
        "default": null
      },
      {
        "name": "status",
        "type": "com.example.complex.UserStatus"
      },
      {
        "name": "preferences",
        "type": "com.example.complex.UserPreferences"
      },
      {
        "name": "scores",
        "type": {
          "type": "array",
          "items": "double"
        }
      },
      {
        "name": "metadata",
        "type": {
          "type": "map",
          "values": "string"
        }
      }
    ]
  }`;

  const userSubject = "com.example.complex.User";
  const userSchemaObject = schemaRegistry.createSchema({
    subject: userSubject,
    schema: userSchema,
    schemaType: SCHEMA_TYPE_AVRO,
    references: [
      {
        name: "com.example.complex.UserStatus",
        subject: userStatusSubject,
        version: userStatusSchemaObject.version,
      },
      {
        name: "com.example.complex.UserPreferences",
        subject: userPreferencesSubject,
        version: userPreferencesSchemaObject.version,
      },
    ],
  });

  // 6. Register OrderItem record
  const orderItemSchema = `{
    "name": "OrderItem",
    "type": "record",
    "namespace": "com.example.complex",
    "fields": [
      {
        "name": "productId",
        "type": "string"
      },
      {
        "name": "quantity",
        "type": "int"
      },
      {
        "name": "price",
        "type": "double"
      },
      {
        "name": "discount",
        "type": ["null", "double"],
        "default": null
      }
    ]
  }`;

  const orderItemSubject = "com.example.complex.OrderItem";
  const orderItemSchemaObject = schemaRegistry.createSchema({
    subject: orderItemSubject,
    schema: orderItemSchema,
    schemaType: SCHEMA_TYPE_AVRO,
  });

  // 7. Register Order record (references Currency, PaymentMethod, and OrderItem)
  const orderSchema = `{
    "name": "Order",
    "type": "record",
    "namespace": "com.example.complex",
    "fields": [
      {
        "name": "orderId",
        "type": "string"
      },
      {
        "name": "items",
        "type": {
          "type": "array",
          "items": "com.example.complex.OrderItem"
        }
      },
      {
        "name": "total",
        "type": "double"
      },
      {
        "name": "currency",
        "type": "com.example.complex.Currency"
      },
      {
        "name": "paymentMethod",
        "type": ["null", "com.example.complex.PaymentMethod"],
        "default": null
      }
    ]
  }`;

  const orderSubject = "com.example.complex.Order";
  const orderSchemaObject = schemaRegistry.createSchema({
    subject: orderSubject,
    schema: orderSchema,
    schemaType: SCHEMA_TYPE_AVRO,
    references: [
      {
        name: "com.example.complex.Currency",
        subject: currencySubject,
        version: currencySchemaObject.version,
      },
      {
        name: "com.example.complex.PaymentMethod",
        subject: paymentMethodSubject,
        version: paymentMethodSchemaObject.version,
      },
      {
        name: "com.example.complex.OrderItem",
        subject: orderItemSubject,
        version: orderItemSchemaObject.version,
      },
    ],
  });

  // 8. Register Coordinates record
  const coordinatesSchema = `{
    "name": "Coordinates",
    "type": "record",
    "namespace": "com.example.complex",
    "fields": [
      {
        "name": "latitude",
        "type": "double"
      },
      {
        "name": "longitude",
        "type": "double"
      }
    ]
  }`;

  const coordinatesSubject = "com.example.complex.Coordinates";
  const coordinatesSchemaObject = schemaRegistry.createSchema({
    subject: coordinatesSubject,
    schema: coordinatesSchema,
    schemaType: SCHEMA_TYPE_AVRO,
  });

  // Now register the key schema
  const keySubjectName = schemaRegistry.getSubjectName({
    topic: topic,
    element: KEY,
    subjectNameStrategy: TOPIC_NAME_STRATEGY,
    schema: keySchema,
  });

  const keySchemaObject = schemaRegistry.createSchema({
    subject: keySubjectName,
    schema: keySchema,
    schemaType: SCHEMA_TYPE_AVRO,
  });

  // Register the main value schema with references to all nested schemas
  const valueSubjectName = schemaRegistry.getSubjectName({
    topic: topic,
    element: VALUE,
    subjectNameStrategy: RECORD_NAME_STRATEGY,
    schema: valueSchema,
  });

  const valueSchemaObject = schemaRegistry.createSchema({
    subject: valueSubjectName,
    schema: valueSchema,
    schemaType: SCHEMA_TYPE_AVRO,
    references: [
      {
        name: "com.example.complex.User",
        subject: userSubject,
        version: userSchemaObject.version,
      },
      {
        name: "com.example.complex.Order",
        subject: orderSubject,
        version: orderSchemaObject.version,
      },
      {
        name: "com.example.complex.Coordinates",
        subject: coordinatesSubject,
        version: coordinatesSchemaObject.version,
      },
      {
        name: "com.example.complex.SHA256",
        subject: sha256Subject,
        version: sha256SchemaObject.version,
      },
    ],
  });

  // Return schema objects for use in default function
  return {
    keySchemaObject,
    valueSchemaObject,
  };
}

// Complex key schema with nested record and union
const keySchema = `{
  "name": "ComplexKey",
  "type": "record",
  "namespace": "com.example.complex",
  "fields": [
    {
      "name": "id",
      "type": "string"
    },
    {
      "name": "metadata",
      "type": {
        "name": "KeyMetadata",
        "type": "record",
        "fields": [
          {
            "name": "source",
            "type": "string"
          },
          {
            "name": "timestamp",
            "type": "int"
          }
        ]
      }
    }
  ]
}`;

// Main value schema that references nested schemas registered in setup()
// This schema uses references to the nested schemas instead of inline definitions
const valueSchema = `{
  "name": "ComplexValue",
  "type": "record",
  "namespace": "com.example.complex",
  "fields": [
    {
      "name": "user",
      "type": "com.example.complex.User"
    },
    {
      "name": "order",
      "type": ["null", "com.example.complex.Order"],
      "default": null
    },
    {
      "name": "tags",
      "type": {
        "type": "array",
        "items": "string"
      }
    },
    {
      "name": "attributes",
      "type": {
        "type": "map",
        "values": ["null", "string", "int", "double", "boolean"]
      }
    },
    {
      "name": "coordinates",
      "type": ["null", "com.example.complex.Coordinates"],
      "default": null
    },
    {
      "name": "timestamp",
      "type": "int"
    },
    {
      "name": "isActive",
      "type": "boolean"
    },
    {
      "name": "version",
      "type": "int"
    },
    {
      "name": "data",
      "type": ["null", "bytes"],
      "default": null
    },
    {
      "name": "hash",
      "type": ["null", "com.example.complex.SHA256"],
      "default": null
    }
  ]
}`;

export default function (data) {
  // Get schema objects from setup()
  const keySchemaObject = data.keySchemaObject;
  const valueSchemaObject = data.valueSchemaObject;
  // Generate multiple complex messages with various combinations
  for (let index = 0; index < 50; index++) {
    // Create complex key with nested record
    const keyData = {
      id: `user-${index}`,
      metadata: {
        source: index % 2 === 0 ? "web" : "mobile",
        timestamp: index,
      },
    };

    // Create complex value with all Avro features
    const valueData = {
      user: {
        id: 1000 + index,
        username: `user${index}`,
        email: index % 3 === 0 ? null : `user${index}@example.com`, // Union type: null or string
        status: ["ACTIVE", "INACTIVE", "SUSPENDED", "PENDING"][index % 4], // Enum
        preferences: {
          theme: index % 2 === 0 ? "dark" : null, // Nested record with nullable field
          notifications: index % 2 === 0,
          tags: [`tag${index}`, `category${index % 3}`], // Array of strings
        },
        scores: [85.5 + index, 90.0 + index, 75.5 + index], // Array of doubles
        metadata: {
          // Map of string to string
          department: `dept${index % 5}`,
          region: `region${index % 3}`,
          custom: `value${index}`,
        },
      },
      order:
        index % 4 === 0
          ? null
          : {
              // Union type: null or Order record
              // Pass the Order record directly - hamba/avro will match it to the union type
              orderId: `order-${index}`,
              items: [
                {
                  productId: `product-${index}-1`,
                  quantity: 2 + index,
                  price: 19.99 + index,
                  discount: index % 3 === 0 ? null : 5.0 + index, // Nullable double
                },
                {
                  productId: `product-${index}-2`,
                  quantity: 1,
                  price: 29.99,
                  discount: null,
                },
              ],
              total: 49.98 + index * 2,
              currency: ["USD", "EUR", "GBP", "JPY"][index % 4], // Enum
              paymentMethod:
                index % 5 === 0
                  ? null
                  : ["CREDIT_CARD", "DEBIT_CARD", "PAYPAL", "BANK_TRANSFER"][
                      index % 4
                    ], // Nullable enum
            },
      tags: [`tag1-${index}`, `tag2-${index}`, `tag3-${index}`], // Array
      attributes: {
        // Map with union values (null, string, int, double, boolean)
        stringAttr: `value${index}`,
        intAttr: index,
        doubleAttr: 3.14159 + index,
        boolAttr: index % 2 === 0,
        nullAttr: null,
      },
      coordinates:
        index % 3 === 0
          ? null
          : {
              // Union type: null or Coordinates record
              // Pass the Coordinates record directly - hamba/avro will match it to the union type
              latitude: 40.7128 + index * 0.001,
              longitude: -74.006 + index * 0.001,
            },
      timestamp: 1000 + index,
      isActive: index % 2 === 0,
      version: 1 + index,
      // For Avro bytes, use array of numbers (0-255) representing byte values
      // In JSON encoding, bytes are typically base64 strings, but arrays work too
      data: index % 5 === 0 ? null : [1, 2, 3, 4, 5, index], // Nullable bytes as array of numbers
      // For Fixed type (SHA256), use array of 32 numbers representing byte values
      hash:
        index % 3 === 0
          ? null
          : Array.from({ length: 32 }, (_, i) => (i + index) % 256),
    };

    let messages = [
      {
        key: schemaRegistry.serialize({
          data: keyData,
          schema: keySchemaObject,
          schemaType: SCHEMA_TYPE_AVRO,
        }),
        value: schemaRegistry.serialize({
          data: valueData,
          schema: valueSchemaObject,
          schemaType: SCHEMA_TYPE_AVRO,
        }),
      },
    ];
    writer.produce({ messages: messages });
  }

  // Consume and verify messages
  let messages = reader.consume({ limit: 20 });
  check(messages, {
    "20 messages returned": (msgs) => msgs.length == 20,
    "key has correct structure": (msgs) => {
      const key = schemaRegistry.deserialize({
        data: msgs[0].key,
        schema: keySchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        key.id !== undefined &&
        key.metadata !== undefined &&
        key.metadata.source !== undefined &&
        key.metadata.timestamp !== undefined
      );
    },
    "value has nested user record": (msgs) => {
      const value = schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        value.user !== undefined &&
        value.user.id !== undefined &&
        value.user.username !== undefined &&
        value.user.status !== undefined
      );
    },
    "value has enum types": (msgs) => {
      const value = schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        ["ACTIVE", "INACTIVE", "SUSPENDED", "PENDING"].includes(
          value.user.status,
        ) &&
        (value.order === null ||
          (value.order !== undefined &&
            ["USD", "EUR", "GBP", "JPY"].includes(value.order.currency)))
      );
    },
    "value has union types (nullable fields)": (msgs) => {
      const value = schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      // email can be null or string
      return value.user.email === null || typeof value.user.email === "string";
    },
    "value has arrays": (msgs) => {
      const value = schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        Array.isArray(value.user.scores) &&
        Array.isArray(value.user.preferences.tags) &&
        Array.isArray(value.tags)
      );
    },
    "value has maps": (msgs) => {
      const value = schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        typeof value.user.metadata === "object" &&
        value.user.metadata !== null &&
        typeof value.attributes === "object" &&
        value.attributes !== null
      );
    },
    "value has nested records": (msgs) => {
      const value = schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        value.user.preferences !== undefined &&
        value.user.preferences.theme !== undefined &&
        value.user.preferences.notifications !== undefined
      );
    },
    "value has deeply nested order items": (msgs) => {
      const value = schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        value.order === null ||
        (value.order !== undefined &&
          Array.isArray(value.order.items) &&
          value.order.items.length > 0 &&
          value.order.items[0].productId !== undefined &&
          value.order.items[0].quantity !== undefined)
      );
    },
    "value has all primitive types": (msgs) => {
      const value = schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        typeof value.timestamp === "number" &&
        typeof value.isActive === "boolean" &&
        typeof value.version === "number"
      );
    },
  });
}

export function teardown(data) {
  const connection = new Connection({
    address: brokers[0],
  });
  connection.deleteTopic(topic);
  connection.close();
  writer.close();
  reader.close();
}
