[**xk6-kafka**](../README.md)

---

# ~~Class: Connection~~

Defined in: [index.d.ts:494](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L494)

## Deprecated

Use `AdminClient` instead. `Connection` remains as a compatibility alias in v2.x.

## Constructors

### Constructor

> **new Connection**(`connectionConfig`): `Connection`

Defined in: [index.d.ts:501](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L501)

#### Parameters

##### connectionConfig

[`ConnectionConfig`](../interfaces/ConnectionConfig.md)

Connection configuration.

#### Returns

`Connection`

- Connection instance.

## Methods

### ~~close()~~

> **close**(): `void`

Defined in: [index.d.ts:527](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L527)

#### Returns

`void`

- Nothing.

#### Destructor

#### Description

Close the connection.

---

### ~~createTopic()~~

> **createTopic**(`topicConfig`): `void`

Defined in: [index.d.ts:508](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L508)

#### Parameters

##### topicConfig

[`TopicConfig`](../interfaces/TopicConfig.md)

Topic configuration.

#### Returns

`void`

- Nothing.

#### Method

Create a new topic.

---

### ~~deleteTopic()~~

> **deleteTopic**(`topic`): `void`

Defined in: [index.d.ts:515](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L515)

#### Parameters

##### topic

`string`

Topic name.

#### Returns

`void`

- Nothing.

#### Method

Delete a topic.

---

### ~~listTopics()~~

> **listTopics**(): `string`[]

Defined in: [index.d.ts:521](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L521)

#### Returns

`string`[]

- Topics.

#### Method

List topics.
