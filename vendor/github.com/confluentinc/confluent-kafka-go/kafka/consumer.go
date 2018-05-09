package kafka

/**
 * Copyright 2016 Confluent Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import (
	"fmt"
	"math"
	"time"
	"unsafe"
)

/*
#include <stdlib.h>
#include <librdkafka/rdkafka.h>


static rd_kafka_topic_partition_t *_c_rdkafka_topic_partition_list_entry(rd_kafka_topic_partition_list_t *rktparlist, int idx) {
   return idx < rktparlist->cnt ? &rktparlist->elems[idx] : NULL;
}
*/
import "C"

// RebalanceCb provides a per-Subscribe*() rebalance event callback.
// The passed Event will be either AssignedPartitions or RevokedPartitions
type RebalanceCb func(*Consumer, Event) error

// Consumer implements a High-level Apache Kafka Consumer instance
type Consumer struct {
	events             chan Event
	handle             handle
	eventsChanEnable   bool
	readerTermChan     chan bool
	rebalanceCb        RebalanceCb
	appReassigned      bool
	appRebalanceEnable bool // config setting
}

// Strings returns a human readable name for a Consumer instance
func (c *Consumer) String() string {
	return c.handle.String()
}

// getHandle implements the Handle interface
func (c *Consumer) gethandle() *handle {
	return &c.handle
}

// Subscribe to a single topic
// This replaces the current subscription
func (c *Consumer) Subscribe(topic string, rebalanceCb RebalanceCb) error {
	return c.SubscribeTopics([]string{topic}, rebalanceCb)
}

// SubscribeTopics subscribes to the provided list of topics.
// This replaces the current subscription.
func (c *Consumer) SubscribeTopics(topics []string, rebalanceCb RebalanceCb) (err error) {
	ctopics := C.rd_kafka_topic_partition_list_new(C.int(len(topics)))
	defer C.rd_kafka_topic_partition_list_destroy(ctopics)

	for _, topic := range topics {
		ctopic := C.CString(topic)
		defer C.free(unsafe.Pointer(ctopic))
		C.rd_kafka_topic_partition_list_add(ctopics, ctopic, C.RD_KAFKA_PARTITION_UA)
	}

	e := C.rd_kafka_subscribe(c.handle.rk, ctopics)
	if e != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return newError(e)
	}

	c.rebalanceCb = rebalanceCb
	c.handle.currAppRebalanceEnable = c.rebalanceCb != nil || c.appRebalanceEnable

	return nil
}

// Unsubscribe from the current subscription, if any.
func (c *Consumer) Unsubscribe() (err error) {
	C.rd_kafka_unsubscribe(c.handle.rk)
	return nil
}

// Assign an atomic set of partitions to consume.
// This replaces the current assignment.
func (c *Consumer) Assign(partitions []TopicPartition) (err error) {
	c.appReassigned = true

	cparts := newCPartsFromTopicPartitions(partitions)
	defer C.rd_kafka_topic_partition_list_destroy(cparts)

	e := C.rd_kafka_assign(c.handle.rk, cparts)
	if e != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return newError(e)
	}

	return nil
}

// Unassign the current set of partitions to consume.
func (c *Consumer) Unassign() (err error) {
	c.appReassigned = true

	e := C.rd_kafka_assign(c.handle.rk, nil)
	if e != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return newError(e)
	}

	return nil
}

// commit offsets for specified offsets.
// If offsets is nil the currently assigned partitions' offsets are committed.
// This is a blocking call, caller will need to wrap in go-routine to
// get async or throw-away behaviour.
func (c *Consumer) commit(offsets []TopicPartition) (committedOffsets []TopicPartition, err error) {
	var rkqu *C.rd_kafka_queue_t

	rkqu = C.rd_kafka_queue_new(c.handle.rk)
	defer C.rd_kafka_queue_destroy(rkqu)

	var coffsets *C.rd_kafka_topic_partition_list_t
	if offsets != nil {
		coffsets = newCPartsFromTopicPartitions(offsets)
		defer C.rd_kafka_topic_partition_list_destroy(coffsets)
	}

	cErr := C.rd_kafka_commit_queue(c.handle.rk, coffsets, rkqu, nil, nil)
	if cErr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return nil, newError(cErr)
	}

	rkev := C.rd_kafka_queue_poll(rkqu, C.int(-1))
	if rkev == nil {
		// shouldn't happen
		return nil, newError(C.RD_KAFKA_RESP_ERR__DESTROY)
	}
	defer C.rd_kafka_event_destroy(rkev)

	if C.rd_kafka_event_type(rkev) != C.RD_KAFKA_EVENT_OFFSET_COMMIT {
		panic(fmt.Sprintf("Expected OFFSET_COMMIT, got %s",
			C.GoString(C.rd_kafka_event_name(rkev))))
	}

	cErr = C.rd_kafka_event_error(rkev)
	if cErr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return nil, newErrorFromCString(cErr, C.rd_kafka_event_error_string(rkev))
	}

	cRetoffsets := C.rd_kafka_event_topic_partition_list(rkev)
	if cRetoffsets == nil {
		// no offsets, no error
		return nil, nil
	}
	committedOffsets = newTopicPartitionsFromCparts(cRetoffsets)

	return committedOffsets, nil
}

// Commit offsets for currently assigned partitions
// This is a blocking call.
// Returns the committed offsets on success.
func (c *Consumer) Commit() ([]TopicPartition, error) {
	return c.commit(nil)
}

// CommitMessage commits offset based on the provided message.
// This is a blocking call.
// Returns the committed offsets on success.
func (c *Consumer) CommitMessage(m *Message) ([]TopicPartition, error) {
	if m.TopicPartition.Error != nil {
		return nil, Error{ErrInvalidArg, "Can't commit errored message"}
	}
	offsets := []TopicPartition{m.TopicPartition}
	offsets[0].Offset++
	return c.commit(offsets)
}

// CommitOffsets commits the provided list of offsets
// This is a blocking call.
// Returns the committed offsets on success.
func (c *Consumer) CommitOffsets(offsets []TopicPartition) ([]TopicPartition, error) {
	return c.commit(offsets)
}

// StoreOffsets stores the provided list of offsets that will be committed
// to the offset store according to `auto.commit.interval.ms` or manual
// offset-less Commit().
//
// Returns the stored offsets on success. If at least one offset couldn't be stored,
// an error and a list of offsets is returned. Each offset can be checked for
// specific errors via its `.Error` member.
func (c *Consumer) StoreOffsets(offsets []TopicPartition) (storedOffsets []TopicPartition, err error) {
	coffsets := newCPartsFromTopicPartitions(offsets)
	defer C.rd_kafka_topic_partition_list_destroy(coffsets)

	cErr := C.rd_kafka_offsets_store(c.handle.rk, coffsets)

	// coffsets might be annotated with an error
	storedOffsets = newTopicPartitionsFromCparts(coffsets)

	if cErr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return storedOffsets, newError(cErr)
	}

	return storedOffsets, nil
}

// Seek seeks the given topic partitions using the offset from the TopicPartition.
//
// If timeoutMs is not 0 the call will wait this long for the
// seek to be performed. If the timeout is reached the internal state
// will be unknown and this function returns ErrTimedOut.
// If timeoutMs is 0 it will initiate the seek but return
// immediately without any error reporting (e.g., async).
//
// Seek() may only be used for partitions already being consumed
// (through Assign() or implicitly through a self-rebalanced Subscribe()).
// To set the starting offset it is preferred to use Assign() and provide
// a starting offset for each partition.
//
// Returns an error on failure or nil otherwise.
func (c *Consumer) Seek(partition TopicPartition, timeoutMs int) error {
	rkt := c.handle.getRkt(*partition.Topic)
	cErr := C.rd_kafka_seek(rkt,
		C.int32_t(partition.Partition),
		C.int64_t(partition.Offset),
		C.int(timeoutMs))
	if cErr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return newError(cErr)
	}
	return nil
}

// Poll the consumer for messages or events.
//
// Will block for at most timeoutMs milliseconds
//
// The following callbacks may be triggered:
//   Subscribe()'s rebalanceCb
//
// Returns nil on timeout, else an Event
func (c *Consumer) Poll(timeoutMs int) (event Event) {
	ev, _ := c.handle.eventPoll(nil, timeoutMs, 1, nil)
	return ev
}

// Events returns the Events channel (if enabled)
func (c *Consumer) Events() chan Event {
	return c.events
}

// ReadMessage polls the consumer for a message.
//
// This is a conveniance API that wraps Poll() and only returns
// messages or errors. All other event types are discarded.
//
// The call will block for at most `timeout` waiting for
// a new message or error. `timeout` may be set to -1 for
// indefinite wait.
//
// Timeout is returned as (nil, err) where err is `kafka.(Error).Code == Kafka.ErrTimedOut`.
//
// Messages are returned as (msg, nil),
// while general errors are returned as (nil, err),
// and partition-specific errors are returned as (msg, err) where
// msg.TopicPartition provides partition-specific information (such as topic, partition and offset).
//
// All other event types, such as PartitionEOF, AssignedPartitions, etc, are silently discarded.
//
func (c *Consumer) ReadMessage(timeout time.Duration) (*Message, error) {

	var absTimeout time.Time
	var timeoutMs int

	if timeout > 0 {
		absTimeout = time.Now().Add(timeout)
		timeoutMs = (int)(timeout.Seconds() * 1000.0)
	} else {
		timeoutMs = (int)(timeout)
	}

	for {
		ev := c.Poll(timeoutMs)

		switch e := ev.(type) {
		case *Message:
			if e.TopicPartition.Error != nil {
				return e, e.TopicPartition.Error
			}
			return e, nil
		case Error:
			return nil, e
		default:
			// Ignore other event types
		}

		if timeout > 0 {
			// Calculate remaining time
			timeoutMs = int(math.Max(0.0, absTimeout.Sub(time.Now()).Seconds()*1000.0))
		}

		if timeoutMs == 0 && ev == nil {
			return nil, newError(C.RD_KAFKA_RESP_ERR__TIMED_OUT)
		}

	}

}

// Close Consumer instance.
// The object is no longer usable after this call.
func (c *Consumer) Close() (err error) {

	if c.eventsChanEnable {
		// Wait for consumerReader() to terminate (by closing readerTermChan)
		close(c.readerTermChan)
		c.handle.waitTerminated(1)
		close(c.events)
	}

	C.rd_kafka_queue_destroy(c.handle.rkq)
	c.handle.rkq = nil

	e := C.rd_kafka_consumer_close(c.handle.rk)
	if e != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return newError(e)
	}

	c.handle.cleanup()

	C.rd_kafka_destroy(c.handle.rk)

	return nil
}

// NewConsumer creates a new high-level Consumer instance.
//
// Supported special configuration properties:
//   go.application.rebalance.enable (bool, false) - Forward rebalancing responsibility to application via the Events() channel.
//                                        If set to true the app must handle the AssignedPartitions and
//                                        RevokedPartitions events and call Assign() and Unassign()
//                                        respectively.
//   go.events.channel.enable (bool, false) - Enable the Events() channel. Messages and events will be pushed on the Events() channel and the Poll() interface will be disabled. (Experimental)
//   go.events.channel.size (int, 1000) - Events() channel size
//
// WARNING: Due to the buffering nature of channels (and queues in general) the
// use of the events channel risks receiving outdated events and
// messages. Minimizing go.events.channel.size reduces the risk
// and number of outdated events and messages but does not eliminate
// the factor completely. With a channel size of 1 at most one
// event or message may be outdated.
func NewConsumer(conf *ConfigMap) (*Consumer, error) {

	err := versionCheck()
	if err != nil {
		return nil, err
	}

	groupid, _ := conf.get("group.id", nil)
	if groupid == nil {
		// without a group.id the underlying cgrp subsystem in librdkafka wont get started
		// and without it there is no way to consume assigned partitions.
		// So for now require the group.id, this might change in the future.
		return nil, newErrorFromString(ErrInvalidArg, "Required property group.id not set")
	}

	c := &Consumer{}

	v, err := conf.extract("go.application.rebalance.enable", false)
	if err != nil {
		return nil, err
	}
	c.appRebalanceEnable = v.(bool)

	v, err = conf.extract("go.events.channel.enable", false)
	if err != nil {
		return nil, err
	}
	c.eventsChanEnable = v.(bool)

	v, err = conf.extract("go.events.channel.size", 1000)
	if err != nil {
		return nil, err
	}
	eventsChanSize := v.(int)

	cConf, err := conf.convert()
	if err != nil {
		return nil, err
	}
	cErrstr := (*C.char)(C.malloc(C.size_t(256)))
	defer C.free(unsafe.Pointer(cErrstr))

	C.rd_kafka_conf_set_events(cConf, C.RD_KAFKA_EVENT_REBALANCE|C.RD_KAFKA_EVENT_OFFSET_COMMIT|C.RD_KAFKA_EVENT_STATS)

	c.handle.rk = C.rd_kafka_new(C.RD_KAFKA_CONSUMER, cConf, cErrstr, 256)
	if c.handle.rk == nil {
		return nil, newErrorFromCString(C.RD_KAFKA_RESP_ERR__INVALID_ARG, cErrstr)
	}

	C.rd_kafka_poll_set_consumer(c.handle.rk)

	c.handle.c = c
	c.handle.setup()
	c.handle.rkq = C.rd_kafka_queue_get_consumer(c.handle.rk)
	if c.handle.rkq == nil {
		// no cgrp (no group.id configured), revert to main queue.
		c.handle.rkq = C.rd_kafka_queue_get_main(c.handle.rk)
	}

	if c.eventsChanEnable {
		c.events = make(chan Event, eventsChanSize)
		c.readerTermChan = make(chan bool)

		/* Start rdkafka consumer queue reader -> events writer goroutine */
		go consumerReader(c, c.readerTermChan)
	}

	return c, nil
}

// rebalance calls the application's rebalance callback, if any.
// Returns true if the underlying assignment was updated, else false.
func (c *Consumer) rebalance(ev Event) bool {
	c.appReassigned = false

	if c.rebalanceCb != nil {
		c.rebalanceCb(c, ev)
	}

	return c.appReassigned
}

// consumerReader reads messages and events from the librdkafka consumer queue
// and posts them on the consumer channel.
// Runs until termChan closes
func consumerReader(c *Consumer, termChan chan bool) {

out:
	for true {
		select {
		case _ = <-termChan:
			break out
		default:
			_, term := c.handle.eventPoll(c.events, 100, 1000, termChan)
			if term {
				break out
			}

		}
	}

	c.handle.terminatedChan <- "consumerReader"
	return

}

// GetMetadata queries broker for cluster and topic metadata.
// If topic is non-nil only information about that topic is returned, else if
// allTopics is false only information about locally used topics is returned,
// else information about all topics is returned.
func (c *Consumer) GetMetadata(topic *string, allTopics bool, timeoutMs int) (*Metadata, error) {
	return getMetadata(c, topic, allTopics, timeoutMs)
}

// QueryWatermarkOffsets returns the broker's low and high offsets for the given topic
// and partition.
func (c *Consumer) QueryWatermarkOffsets(topic string, partition int32, timeoutMs int) (low, high int64, err error) {
	return queryWatermarkOffsets(c, topic, partition, timeoutMs)
}

// OffsetsForTimes looks up offsets by timestamp for the given partitions.
//
// The returned offset for each partition is the earliest offset whose
// timestamp is greater than or equal to the given timestamp in the
// corresponding partition.
//
// The timestamps to query are represented as `.Offset` in the `times`
// argument and the looked up offsets are represented as `.Offset` in the returned
// `offsets` list.
//
// The function will block for at most timeoutMs milliseconds.
//
// Duplicate Topic+Partitions are not supported.
// Per-partition errors may be returned in the `.Error` field.
func (c *Consumer) OffsetsForTimes(times []TopicPartition, timeoutMs int) (offsets []TopicPartition, err error) {
	return offsetsForTimes(c, times, timeoutMs)
}

// Subscription returns the current subscription as set by Subscribe()
func (c *Consumer) Subscription() (topics []string, err error) {
	var cTopics *C.rd_kafka_topic_partition_list_t

	cErr := C.rd_kafka_subscription(c.handle.rk, &cTopics)
	if cErr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return nil, newError(cErr)
	}
	defer C.rd_kafka_topic_partition_list_destroy(cTopics)

	topicCnt := int(cTopics.cnt)
	topics = make([]string, topicCnt)
	for i := 0; i < topicCnt; i++ {
		crktpar := C._c_rdkafka_topic_partition_list_entry(cTopics,
			C.int(i))
		topics[i] = C.GoString(crktpar.topic)
	}

	return topics, nil
}

// Assignment returns the current partition assignments
func (c *Consumer) Assignment() (partitions []TopicPartition, err error) {
	var cParts *C.rd_kafka_topic_partition_list_t

	cErr := C.rd_kafka_assignment(c.handle.rk, &cParts)
	if cErr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return nil, newError(cErr)
	}
	defer C.rd_kafka_topic_partition_list_destroy(cParts)

	partitions = newTopicPartitionsFromCparts(cParts)

	return partitions, nil
}

// Committed retrieves committed offsets for the given set of partitions
func (c *Consumer) Committed(partitions []TopicPartition, timeoutMs int) (offsets []TopicPartition, err error) {
	cparts := newCPartsFromTopicPartitions(partitions)
	defer C.rd_kafka_topic_partition_list_destroy(cparts)
	cerr := C.rd_kafka_committed(c.handle.rk, cparts, C.int(timeoutMs))
	if cerr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return nil, newError(cerr)
	}

	return newTopicPartitionsFromCparts(cparts), nil
}

// Pause consumption for the provided list of partitions
//
// Note that messages already enqueued on the consumer's Event channel
// (if `go.events.channel.enable` has been set) will NOT be purged by
// this call, set `go.events.channel.size` accordingly.
func (c *Consumer) Pause(partitions []TopicPartition) (err error) {
	cparts := newCPartsFromTopicPartitions(partitions)
	defer C.rd_kafka_topic_partition_list_destroy(cparts)
	cerr := C.rd_kafka_pause_partitions(c.handle.rk, cparts)
	if cerr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return newError(cerr)
	}
	return nil
}

// Resume consumption for the provided list of partitions
func (c *Consumer) Resume(partitions []TopicPartition) (err error) {
	cparts := newCPartsFromTopicPartitions(partitions)
	defer C.rd_kafka_topic_partition_list_destroy(cparts)
	cerr := C.rd_kafka_resume_partitions(c.handle.rk, cparts)
	if cerr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return newError(cerr)
	}
	return nil
}
