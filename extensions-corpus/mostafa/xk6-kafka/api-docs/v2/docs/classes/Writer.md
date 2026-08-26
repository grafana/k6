[**xk6-kafka**](../README.md)

---

# ~~Class: Writer~~

Defined in: [index.d.ts:386](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L386)

## Deprecated

Use `Producer` instead. `Writer` remains as a compatibility alias in v2.x.

## Constructors

### Constructor

> **new Writer**(`writerConfig`): `Writer`

Defined in: [index.d.ts:393](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L393)

#### Parameters

##### writerConfig

[`WriterConfig`](../interfaces/WriterConfig.md)

Writer configuration.

#### Returns

`Writer`

- Writer instance.

## Methods

### ~~close()~~

> **close**(): `void`

Defined in: [index.d.ts:406](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L406)

#### Returns

`void`

- Nothing.

#### Destructor

#### Description

Close the writer.

---

### ~~produce()~~

> **produce**(`produceConfig`): `void`

Defined in: [index.d.ts:400](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L400)

#### Parameters

##### produceConfig

[`ProduceConfig`](../interfaces/ProduceConfig.md)

Produce configuration.

#### Returns

`void`

- Nothing.

#### Method

Write messages to Kafka.
