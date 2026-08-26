[**xk6-kafka**](../README.md)

---

# Interface: ConsumeConfig

Defined in: [index.d.ts:226](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L226)

Configuration for Consume method.

## Properties

### expectTimeout

> **expectTimeout**: `boolean`

Defined in: [index.d.ts:237](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L237)

If true, return whatever messages have been collected when maxWait is
passed.

---

### limit?

> `optional` **limit?**: `number`

Defined in: [index.d.ts:228](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L228)

collect this many messages before returning.

---

### maxMessages?

> `optional` **maxMessages?**: `number`

Defined in: [index.d.ts:230](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L230)

preferred v2 alias for limit.

---

### nanoPrecision

> **nanoPrecision**: `boolean`

Defined in: [index.d.ts:232](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L232)

If true, returned message RFC3339 timestamps carry nanosecond precision.
