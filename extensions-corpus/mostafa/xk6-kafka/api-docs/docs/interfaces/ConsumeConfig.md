[**xk6-kafka**](../README.md)

---

# Interface: ConsumeConfig

Defined in: [index.d.ts:211](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L211)

Configuration for Consume method.

## Properties

### expectTimeout

> **expectTimeout**: `boolean`

Defined in: [index.d.ts:220](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L220)

If true, return whatever messages have been collected when maxWait is
passed.

---

### limit

> **limit**: `number`

Defined in: [index.d.ts:213](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L213)

collect this many messages before returning.

---

### nanoPrecision

> **nanoPrecision**: `boolean`

Defined in: [index.d.ts:215](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L215)

If true, returned message RFC3339 timestamps carry nanosecond precision.
