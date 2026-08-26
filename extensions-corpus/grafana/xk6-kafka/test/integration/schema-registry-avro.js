// Integration test: Avro serdes with Schema Registry.
// Registers an Avro schema, produces with it, and consumes to verify round-trip.

import { Writer, Reader, Connection, SchemaRegistry, SCHEMA_TYPE_AVRO } from "k6/x/kafka";
import { thresholds, getBroker, getSchemaRegistry, verify, runTest, uniqueTopic } from "./lib/common.js";

export const options = { thresholds };

export default function () {
  runTest(() => {
    const srURL = getSchemaRegistry();
    if (!srURL) {
      verify("schema registry available", false);
      return;
    }

    const broker = getBroker();
    const topic = uniqueTopic("xk6_avro");
    const avroSchema = JSON.stringify({
      type: "record",
      name: "User",
      fields: [
        { name: "id", type: "int" },
        { name: "name", type: "string" },
      ],
    });

    const connection = new Connection({ address: broker });
    try {
      connection.createTopic({ topic, numPartitions: 1, replicationFactor: 1 });

      const sr = new SchemaRegistry({ url: srURL });
      const schema = sr.createSchema({
        subject: `${topic}-value`,
        schema: avroSchema,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      verify("created schema", schema && schema.id > 0);

      const writer = new Writer({ brokers: [broker], topic });
      try {
        writer.produce({
          messages: [
            {
              value: sr.serialize({
                data: { id: 1, name: "Alice" },
                schemaType: SCHEMA_TYPE_AVRO,
                schema: schema,
              }),
            },
          ],
        });
        verify("produced avro message", true);

        const reader = new Reader({ brokers: [broker], topic, groupID: "xk6_avro" });
        try {
          const messages = reader.consume({ limit: 1 });
          verify("consumed message", messages.length === 1);
          if (messages.length > 0) {
            const decoded = sr.deserialize({
              data: messages[0].value,
              schemaType: SCHEMA_TYPE_AVRO,
              schema: schema,
            });
            verify("decoded avro", decoded && decoded.id === 1 && decoded.name === "Alice");
          }
        } finally {
          reader.close();
        }
      } finally {
        writer.close();
      }

      try {
        connection.deleteTopic(topic);
      } catch (e) {
        // Ignore topic deletion errors
      }
    } finally {
      connection.close();
    }
  });
}
