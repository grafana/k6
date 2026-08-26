[**xk6-kafka**](../README.md)

---

# Class: Producer

Defined in: [index.d.ts:375](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L375)

## Classdesc

Producer writes messages to Kafka.

## Example

```javascript
// In init context
const producer = new Producer({
  brokers: ["localhost:9092"],
  topic: "my-topic",
  autoCreateTopic: true,
});

// In VU code (default function)
producer.produce({
  messages: [
    {
      key: "key",
      value: "value",
    },
  ],
});

// In teardown function
producer.close();
```

## Constructors

### Constructor

> **new Producer**(`writerConfig`): `Producer`

Defined in: [index.d.ts:376](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L376)

#### Parameters

##### writerConfig

[`WriterConfig`](../interfaces/WriterConfig.md)

#### Returns

`Producer`

## Methods

### close()

> **close**(): `void`

Defined in: [index.d.ts:380](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L380)

#### Returns

`void`

---

### flush()

> **flush**(): `void`

Defined in: [index.d.ts:378](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L378)

#### Returns

`void`

---

### produce()

> **produce**(`produceConfig`): `void`

Defined in: [index.d.ts:377](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L377)

#### Parameters

##### produceConfig

[`ProduceConfig`](../interfaces/ProduceConfig.md)

#### Returns

`void`

---

### stats()

> **stats**(): [`ProducerStats`](../interfaces/ProducerStats.md)

Defined in: [index.d.ts:379](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L379)

#### Returns

[`ProducerStats`](../interfaces/ProducerStats.md)
