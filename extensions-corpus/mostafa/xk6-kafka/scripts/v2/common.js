import { sleep } from "k6";

const defaultBroker = "localhost:9092";
const defaultSchemaRegistryURL = "http://localhost:8081";
const defaultTopicPrefix = "xk6-kafka-v2";

export const brokers = (__ENV.XK6_KAFKA_BROKERS || defaultBroker)
  .split(",")
  .map((broker) => broker.trim())
  .filter(Boolean);

export const schemaRegistryURL =
  __ENV.XK6_KAFKA_SCHEMA_REGISTRY_URL || defaultSchemaRegistryURL;

const runId = sanitizeName(__ENV.XK6_KAFKA_RUN_ID || `${Date.now()}`);

export function topicName(suffix) {
  return `${defaultTopicPrefix}-${sanitizeName(suffix)}-${runId}`;
}

export function createTopic(adminClient, topic, overrides = {}) {
  adminClient.createTopic({
    topic,
    ...overrides,
  });

  waitForTopic(adminClient, topic);
}

export function deleteTopic(adminClient, topic) {
  const deadline = Date.now() + 10 * 1000;

  while (Date.now() < deadline) {
    try {
      adminClient.deleteTopic(topic);
    } catch (error) {
      if (!isUnknownTopicError(error)) {
        throw error;
      }
    }

    if (!topicExists(adminClient, topic)) {
      return;
    }

    sleep(0.25);
  }

  throw new Error(`Timed out waiting for topic deletion ${topic}`);
}

export function decodeBytes(bytes) {
  return String.fromCharCode(...bytes);
}

function waitForTopic(adminClient, topic, timeoutSeconds = 10) {
  const deadline = Date.now() + timeoutSeconds * 1000;

  while (Date.now() < deadline) {
    const topics = adminClient.listTopics();
    if (topics.some((entry) => topicEntryName(entry) === topic)) {
      return;
    }

    sleep(0.25);
  }

  throw new Error(`Timed out waiting for topic ${topic}`);
}

function topicEntryName(entry) {
  if (typeof entry === "string") {
    return entry;
  }

  return entry.topic || entry.Topic || entry.name || entry.Name || "";
}

function topicExists(adminClient, topic) {
  const topics = adminClient.listTopics();
  return topics.some((entry) => topicEntryName(entry) === topic);
}

function isUnknownTopicError(error) {
  return /unknown topic|unknown topic or partition/i.test(String(error));
}

function sanitizeName(value) {
  return String(value).replace(/[^a-zA-Z0-9._-]+/g, "-");
}
