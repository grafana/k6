[**xk6-kafka**](../README.md)

---

# Class: Consumer

Defined in: [index.d.ts:428](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L428)

## Classdesc

Consumer reads messages from Kafka.

## Example

```javascript
// In init context
const consumer = new Consumer({
  brokers: ["localhost:9092"],
  topic: "my-topic",
});

// In VU code (default function)
const messages = consumer.consume({ maxMessages: 10, nanoPrecision: false });

// In teardown function
consumer.close();
```

## Constructors

### Constructor

> **new Consumer**(`readerConfig`): `Consumer`

Defined in: [index.d.ts:429](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L429)

#### Parameters

##### readerConfig

[`ReaderConfig`](../interfaces/ReaderConfig.md)

#### Returns

`Consumer`

## Methods

### close()

> **close**(): `void`

Defined in: [index.d.ts:435](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L435)

#### Returns

`void`

---

### commitOffsets()

> **commitOffsets**(): `void`

Defined in: [index.d.ts:433](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L433)

#### Returns

`void`

---

### consume()

> **consume**(`consumeConfig`): [`Message`](../interfaces/Message.md)[]

Defined in: [index.d.ts:430](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L430)

#### Parameters

##### consumeConfig

[`ConsumeConfig`](../interfaces/ConsumeConfig.md)

#### Returns

[`Message`](../interfaces/Message.md)[]

---

### position()

> **position**(`partition`): `number`

Defined in: [index.d.ts:432](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L432)

#### Parameters

##### partition

`number`

#### Returns

`number`

---

### seek()

> **seek**(`partition`, `offset`): `void`

Defined in: [index.d.ts:431](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L431)

#### Parameters

##### partition

`number`

##### offset

`number`

#### Returns

`void`

---

### stats()

> **stats**(): [`ConsumerStats`](../interfaces/ConsumerStats.md)

Defined in: [index.d.ts:434](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L434)

#### Returns

[`ConsumerStats`](../interfaces/ConsumerStats.md)
