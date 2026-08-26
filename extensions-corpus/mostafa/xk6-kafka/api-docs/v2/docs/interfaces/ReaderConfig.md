[**xk6-kafka**](../README.md)

---

# Interface: ReaderConfig

Defined in: [index.d.ts:191](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L191)

## Properties

### brokers

> **brokers**: `string`[]

Defined in: [index.d.ts:192](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L192)

---

### commitInterval

> **commitInterval**: `number`

Defined in: [index.d.ts:207](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L207)

---

### connectLogger

> **connectLogger**: `boolean`

Defined in: [index.d.ts:217](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L217)

---

### groupBalancers

> **groupBalancers**: [`GROUP_BALANCERS`](../enumerations/GROUP_BALANCERS.md)[]

Defined in: [index.d.ts:205](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L205)

---

### groupId

> **groupId**: `string`

Defined in: [index.d.ts:193](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L193)

---

### ~~groupID?~~

> `optional` **groupID?**: `string`

Defined in: [index.d.ts:195](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L195)

#### Deprecated

Use `groupId` instead.

---

### groupTopics

> **groupTopics**: `string`[]

Defined in: [index.d.ts:196](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L196)

---

### heartbeatInterval

> **heartbeatInterval**: `number`

Defined in: [index.d.ts:206](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L206)

---

### isolationLevel

> **isolationLevel**: [`ISOLATION_LEVEL`](../enumerations/ISOLATION_LEVEL.md)

Defined in: [index.d.ts:219](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L219)

---

### joinGroupBackoff

> **joinGroupBackoff**: `number`

Defined in: [index.d.ts:212](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L212)

---

### maxAttempts

> **maxAttempts**: `number`

Defined in: [index.d.ts:218](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L218)

---

### maxBytes

> **maxBytes**: `number`

Defined in: [index.d.ts:201](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L201)

---

### maxWait

> **maxWait**: `string`

Defined in: [index.d.ts:203](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L203)

---

### minBytes

> **minBytes**: `number`

Defined in: [index.d.ts:200](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L200)

---

### offset

> **offset**: `number`

Defined in: [index.d.ts:220](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L220)

---

### partition

> **partition**: `number`

Defined in: [index.d.ts:198](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L198)

---

### partitionWatchInterval

> **partitionWatchInterval**: `number`

Defined in: [index.d.ts:208](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L208)

---

### queueCapacity

> **queueCapacity**: `number`

Defined in: [index.d.ts:199](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L199)

---

### readBackoffMax

> **readBackoffMax**: `number`

Defined in: [index.d.ts:216](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L216)

---

### readBackoffMin

> **readBackoffMin**: `number`

Defined in: [index.d.ts:215](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L215)

---

### readBatchTimeout

> **readBatchTimeout**: `number`

Defined in: [index.d.ts:202](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L202)

---

### readLagInterval

> **readLagInterval**: `number`

Defined in: [index.d.ts:204](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L204)

---

### rebalanceTimeout

> **rebalanceTimeout**: `number`

Defined in: [index.d.ts:211](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L211)

---

### retentionTime

> **retentionTime**: `number`

Defined in: [index.d.ts:213](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L213)

---

### sasl

> **sasl**: [`SASLConfig`](SASLConfig.md)

Defined in: [index.d.ts:221](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L221)

---

### sessionTimeout

> **sessionTimeout**: `number`

Defined in: [index.d.ts:210](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L210)

---

### startOffset

> **startOffset**: [`START_OFFSETS`](../enumerations/START_OFFSETS.md)

Defined in: [index.d.ts:214](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L214)

---

### tls

> **tls**: [`TLSConfig`](TLSConfig.md)

Defined in: [index.d.ts:222](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L222)

---

### topic

> **topic**: `string`

Defined in: [index.d.ts:197](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L197)

---

### watchPartitionChanges

> **watchPartitionChanges**: `boolean`

Defined in: [index.d.ts:209](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts#L209)
