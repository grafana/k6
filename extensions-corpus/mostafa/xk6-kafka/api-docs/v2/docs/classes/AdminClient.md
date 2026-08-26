[**xk6-kafka**](../README.md)

---

# Class: AdminClient

Defined in: [index.d.ts:482](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L482)

## Classdesc

AdminClient connects to Kafka for topic administration.

## Example

```javascript
// In init context
const adminClient = new AdminClient({
  brokers: ["localhost:9092"],
});

// In VU code (default function)
const topics = adminClient.listTopics();

// In teardown function
adminClient.close();
```

## Constructors

### Constructor

> **new AdminClient**(`connectionConfig`): `AdminClient`

Defined in: [index.d.ts:483](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L483)

#### Parameters

##### connectionConfig

[`ConnectionConfig`](../interfaces/ConnectionConfig.md)

#### Returns

`AdminClient`

## Methods

### close()

> **close**(): `void`

Defined in: [index.d.ts:488](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L488)

#### Returns

`void`

---

### createTopic()

> **createTopic**(`topicConfig`): `void`

Defined in: [index.d.ts:484](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L484)

#### Parameters

##### topicConfig

[`TopicConfig`](../interfaces/TopicConfig.md)

#### Returns

`void`

---

### deleteTopic()

> **deleteTopic**(`topic`): `void`

Defined in: [index.d.ts:485](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L485)

#### Parameters

##### topic

`string`

#### Returns

`void`

---

### getMetadata()

> **getMetadata**(`topic`): [`TopicMetadata`](../interfaces/TopicMetadata.md)

Defined in: [index.d.ts:487](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L487)

#### Parameters

##### topic

`string`

#### Returns

[`TopicMetadata`](../interfaces/TopicMetadata.md)

---

### listTopics()

> **listTopics**(): [`TopicInfo`](../interfaces/TopicInfo.md)[]

Defined in: [index.d.ts:486](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L486)

#### Returns

[`TopicInfo`](../interfaces/TopicInfo.md)[]
