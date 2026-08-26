// Integration test: SASL/PLAIN authentication against a secured (SASL_PLAINTEXT)
// listener. Exercises authenticated connect + admin + produce/consume, and that
// bad credentials are rejected.
//
// Requires KAFKA_SASL_BROKER (a SASL_PLAINTEXT listener); `make integration` and
// CI set it to the compose SASL port. Skips when unset.
import { Writer, Reader, Connection, SASL_PLAIN, START_OFFSETS_FIRST_OFFSET } from "k6/x/kafka";
import {
  thresholds, getSaslBroker, saslUser, saslPass, verify, runTest, toStr, uniqueTopic,
} from "./lib/common.js";

export const options = { thresholds };

export default function () {
  runTest(() => {
    const broker = getSaslBroker();
    if (!broker) {
      console.log("KAFKA_SASL_BROKER not set; skipping SASL test");
      return;
    }
    const sasl = { username: saslUser, password: saslPass, algorithm: SASL_PLAIN };
    const topic = uniqueTopic("xk6_sasl");

    const connection = new Connection({ address: broker, sasl });
    try {
      connection.createTopic({ topic, numPartitions: 1, replicationFactor: 1 });
      try {
        const writer = new Writer({ brokers: [broker], topic, sasl });
        try {
          writer.produce({ messages: [{ key: "k", value: "v" }] });
        } finally {
          writer.close();
        }

        const reader = new Reader({
          brokers: [broker], topic, partition: 0,
          startOffset: START_OFFSETS_FIRST_OFFSET, sasl,
        });
        let got = [];
        try {
          for (let i = 0; i < 5 && got.length < 1; i++) {
            got = got.concat(reader.consume({ limit: 1, expectTimeout: true }));
          }
        } finally {
          reader.close();
        }
        verify("SASL round-trip", got.length >= 1 && toStr(got[0].value) === "v");
      } finally {
        connection.deleteTopic(topic);
      }
    } finally {
      connection.close();
    }

    // Negative: wrong credentials must fail to connect (Connection pings eagerly).
    let rejected = false;
    try {
      const bad = new Connection({ address: broker, sasl: { ...sasl, password: "wrong" } });
      bad.close();
    } catch (_e) {
      rejected = true;
    }
    verify("SASL rejects bad credentials", rejected);
  });
}
