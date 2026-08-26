[**xk6-kafka**](../README.md)

---

# Class: Writer

Defined in: [index.d.ts:325](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L325)

## Classdesc

Writer writes messages to Kafka.

## Example

```javascript
// In init context
const writer = new Writer({
  brokers: ["localhost:9092"],
  topic: "my-topic",
  autoCreateTopic: true,
});

// In VU code (default function)
writer.produce({
  messages: [
    {
      key: "key",
      value: "value",
    },
  ],
});

// In teardown function
writer.close();
```

## Constructors

### Constructor

> **new Writer**(`writerConfig`): `Writer`

Defined in: [index.d.ts:332](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L332)

#### Parameters

##### writerConfig

[`WriterConfig`](../interfaces/WriterConfig.md)

Writer configuration.

#### Returns

`Writer`

- Writer instance.

## Methods

### close()

> **close**(): `void`

Defined in: [index.d.ts:345](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L345)

#### Returns

`void`

- Nothing.

#### Destructor

#### Description

Close the writer.

---

### produce()

> **produce**(`produceConfig`): `void`

Defined in: [index.d.ts:339](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L339)

#### Parameters

##### produceConfig

[`ProduceConfig`](../interfaces/ProduceConfig.md)

Produce configuration.

#### Returns

`void`

- Nothing.

#### Method

Write messages to Kafka.
