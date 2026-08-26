[**xk6-kafka**](../README.md)

---

# Interface: ReaderConfig

Defined in: [index.d.ts:178](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L178)

## Properties

### brokers

> **brokers**: `string`[]

Defined in: [index.d.ts:179](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L179)

---

### commitInterval

> **commitInterval**: `number`

Defined in: [index.d.ts:192](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L192)

---

### connectLogger

> **connectLogger**: `boolean`

Defined in: [index.d.ts:202](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L202)

---

### groupBalancers

> **groupBalancers**: [`GROUP_BALANCERS`](../enumerations/GROUP_BALANCERS.md)[]

Defined in: [index.d.ts:190](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L190)

---

### groupId

> **groupId**: `string`

Defined in: [index.d.ts:180](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L180)

---

### groupID

> `optional` **groupID**: `string`

Deprecated. Use `groupId` instead.

Defined in: [index.d.ts:182](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L182)

---

### groupTopics

> **groupTopics**: `string`[]

Defined in: [index.d.ts:181](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L181)

---

### heartbeatInterval

> **heartbeatInterval**: `number`

Defined in: [index.d.ts:191](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L191)

---

### isolationLevel

> **isolationLevel**: [`ISOLATION_LEVEL`](../enumerations/ISOLATION_LEVEL.md)

Defined in: [index.d.ts:204](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L204)

---

### joinGroupBackoff

> **joinGroupBackoff**: `number`

Defined in: [index.d.ts:197](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L197)

---

### maxAttempts

> **maxAttempts**: `number`

Defined in: [index.d.ts:203](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L203)

---

### maxBytes

> **maxBytes**: `number`

Defined in: [index.d.ts:186](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L186)

---

### maxWait

> **maxWait**: `string`

Defined in: [index.d.ts:188](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L188)

---

### minBytes

> **minBytes**: `number`

Defined in: [index.d.ts:185](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L185)

---

### offset

> **offset**: `number`

Defined in: [index.d.ts:205](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L205)

---

### partition

> **partition**: `number`

Defined in: [index.d.ts:183](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L183)

---

### partitionWatchInterval

> **partitionWatchInterval**: `number`

Defined in: [index.d.ts:193](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L193)

---

### queueCapacity

> **queueCapacity**: `number`

Defined in: [index.d.ts:184](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L184)

---

### readBackoffMax

> **readBackoffMax**: `number`

Defined in: [index.d.ts:201](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L201)

---

### readBackoffMin

> **readBackoffMin**: `number`

Defined in: [index.d.ts:200](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L200)

---

### readBatchTimeout

> **readBatchTimeout**: `number`

Defined in: [index.d.ts:187](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L187)

---

### readLagInterval

> **readLagInterval**: `number`

Defined in: [index.d.ts:189](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L189)

---

### rebalanceTimeout

> **rebalanceTimeout**: `number`

Defined in: [index.d.ts:196](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L196)

---

### retentionTime

> **retentionTime**: `number`

Defined in: [index.d.ts:198](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L198)

---

### sasl

> **sasl**: [`SASLConfig`](SASLConfig.md)

Defined in: [index.d.ts:206](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L206)

---

### sessionTimeout

> **sessionTimeout**: `number`

Defined in: [index.d.ts:195](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L195)

---

### startOffset

> **startOffset**: [`START_OFFSETS`](../enumerations/START_OFFSETS.md)

Defined in: [index.d.ts:199](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L199)

---

### tls

> **tls**: [`TLSConfig`](TLSConfig.md)

Defined in: [index.d.ts:207](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L207)

---

### topic

> **topic**: `string`

Defined in: [index.d.ts:182](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L182)

---

### watchPartitionChanges

> **watchPartitionChanges**: `boolean`

Defined in: [index.d.ts:194](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L194)
