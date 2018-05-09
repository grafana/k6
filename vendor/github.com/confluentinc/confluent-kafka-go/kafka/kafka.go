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

// Package kafka provides high-level Apache Kafka producer and consumers
// using bindings on-top of the librdkafka C library.
//
//
// High-level Consumer
//
// * Decide if you want to read messages and events from the `.Events()` channel
// (set `"go.events.channel.enable": true`) or by calling `.Poll()`.
//
// * Create a Consumer with `kafka.NewConsumer()` providing at
// least the `bootstrap.servers` and `group.id` configuration properties.
//
// * Call `.Subscribe()` or (`.SubscribeTopics()` to subscribe to multiple topics)
// to join the group with the specified subscription set.
// Subscriptions are atomic, calling `.Subscribe*()` again will leave
// the group and rejoin with the new set of topics.
//
// * Start reading events and messages from either the `.Events` channel
// or by calling `.Poll()`.
//
// * When the group has rebalanced each client member is assigned a
// (sub-)set of topic+partitions.
// By default the consumer will start fetching messages for its assigned
// partitions at this point, but your application may enable rebalance
// events to get an insight into what the assigned partitions where
// as well as set the initial offsets. To do this you need to pass
// `"go.application.rebalance.enable": true` to the `NewConsumer()` call
// mentioned above. You will (eventually) see a `kafka.AssignedPartitions` event
// with the assigned partition set. You can optionally modify the initial
// offsets (they'll default to stored offsets and if there are no previously stored
// offsets it will fall back to `"default.topic.config": ConfigMap{"auto.offset.reset": ..}`
// which defaults to the `latest` message) and then call `.Assign(partitions)`
// to start consuming. If you don't need to modify the initial offsets you will
// not need to call `.Assign()`, the client will do so automatically for you if
// you dont.
//
// * As messages are fetched they will be made available on either the
// `.Events` channel or by calling `.Poll()`, look for event type `*kafka.Message`.
//
// * Handle messages, events and errors to your liking.
//
// * When you are done consuming call `.Close()` to commit final offsets
// and leave the consumer group.
//
//
//
// Producer
//
// * Create a Producer with `kafka.NewProducer()` providing at least
// the `bootstrap.servers` configuration properties.
//
// * Messages may now be produced either by sending a `*kafka.Message`
// on the `.ProduceChannel` or by calling `.Produce()`.
//
// * Producing is an asynchronous operation so the client notifies the application
// of per-message produce success or failure through something called delivery reports.
// Delivery reports are by default emitted on the `.Events()` channel as `*kafka.Message`
// and you should check `msg.TopicPartition.Error` for `nil` to find out if the message
// was succesfully delivered or not.
// It is also possible to direct delivery reports to alternate channels
// by providing a non-nil `chan Event` channel to `.Produce()`.
// If no delivery reports are wanted they can be completely disabled by
// setting configuration property `"go.delivery.reports": false`.
//
// * When you are done producing messages you will need to make sure all messages
// are indeed delivered to the broker (or failed), remember that this is
// an asynchronous client so some of your messages may be lingering in internal
// channels or tranmission queues.
// To do this you can either keep track of the messages you've produced
// and wait for their corresponding delivery reports, or call the convenience
// function `.Flush()` that will block until all message deliveries are done
// or the provided timeout elapses.
//
// * Finally call `.Close()` to decommission the producer.
//
//
// Events
//
// Apart from emitting messages and delivery reports the client also communicates
// with the application through a number of different event types.
// An application may choose to handle or ignore these events.
//
// Consumer events
//
// * `*kafka.Message` - a fetched message.
//
// * `AssignedPartitions` - The assigned partition set for this client following a rebalance.
// Requires `go.application.rebalance.enable`
//
// * `RevokedPartitions` - The counter part to `AssignedPartitions` following a rebalance.
// `AssignedPartitions` and `RevokedPartitions` are symetrical.
// Requires `go.application.rebalance.enable`
//
// * `PartitionEOF` - Consumer has reached the end of a partition.
// NOTE: The consumer will keep trying to fetch new messages for the partition.
//
// * `OffsetsCommitted` - Offset commit results (when `enable.auto.commit` is enabled).
//
//
// Producer events
//
// * `*kafka.Message` - delivery report for produced message.
// Check `.TopicPartition.Error` for delivery result.
//
//
// Generic events for both Consumer and Producer
//
// * `KafkaError` - client (error codes are prefixed with _) or broker error.
// These errors are normally just informational since the
// client will try its best to automatically recover (eventually).
//
//
// Hint: If your application registers a signal notification
// (signal.Notify) makes sure the signals channel is buffered to avoid
// possible complications with blocking Poll() calls.
//
// Note: The Confluent Kafka Go client is safe for concurrent use.
package kafka

import (
	"fmt"
	"unsafe"
)

/*
#include <stdlib.h>
#include <string.h>
#include <librdkafka/rdkafka.h>

static rd_kafka_topic_partition_t *_c_rdkafka_topic_partition_list_entry(rd_kafka_topic_partition_list_t *rktparlist, int idx) {
   return idx < rktparlist->cnt ? &rktparlist->elems[idx] : NULL;
}
*/
import "C"

// PartitionAny represents any partition (for partitioning),
// or unspecified value (for all other cases)
const PartitionAny = int32(C.RD_KAFKA_PARTITION_UA)

// TopicPartition is a generic placeholder for a Topic+Partition and optionally Offset.
type TopicPartition struct {
	Topic     *string
	Partition int32
	Offset    Offset
	Error     error
}

func (p TopicPartition) String() string {
	topic := "<null>"
	if p.Topic != nil {
		topic = *p.Topic
	}
	if p.Error != nil {
		return fmt.Sprintf("%s[%d]@%s(%s)",
			topic, p.Partition, p.Offset, p.Error)
	}
	return fmt.Sprintf("%s[%d]@%s",
		topic, p.Partition, p.Offset)
}

// TopicPartitions is a slice of TopicPartitions that also implements
// the sort interface
type TopicPartitions []TopicPartition

func (tps TopicPartitions) Len() int {
	return len(tps)
}

func (tps TopicPartitions) Less(i, j int) bool {
	if *tps[i].Topic < *tps[j].Topic {
		return true
	} else if *tps[i].Topic > *tps[j].Topic {
		return false
	}
	return tps[i].Partition < tps[j].Partition
}

func (tps TopicPartitions) Swap(i, j int) {
	tps[i], tps[j] = tps[j], tps[i]
}

// new_cparts_from_TopicPartitions creates a new C rd_kafka_topic_partition_list_t
// from a TopicPartition array.
func newCPartsFromTopicPartitions(partitions []TopicPartition) (cparts *C.rd_kafka_topic_partition_list_t) {
	cparts = C.rd_kafka_topic_partition_list_new(C.int(len(partitions)))
	for _, part := range partitions {
		ctopic := C.CString(*part.Topic)
		defer C.free(unsafe.Pointer(ctopic))
		rktpar := C.rd_kafka_topic_partition_list_add(cparts, ctopic, C.int32_t(part.Partition))
		rktpar.offset = C.int64_t(part.Offset)
	}

	return cparts
}

func setupTopicPartitionFromCrktpar(partition *TopicPartition, crktpar *C.rd_kafka_topic_partition_t) {

	topic := C.GoString(crktpar.topic)
	partition.Topic = &topic
	partition.Partition = int32(crktpar.partition)
	partition.Offset = Offset(crktpar.offset)
	if crktpar.err != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		partition.Error = newError(crktpar.err)
	}
}

func newTopicPartitionsFromCparts(cparts *C.rd_kafka_topic_partition_list_t) (partitions []TopicPartition) {

	partcnt := int(cparts.cnt)

	partitions = make([]TopicPartition, partcnt)
	for i := 0; i < partcnt; i++ {
		crktpar := C._c_rdkafka_topic_partition_list_entry(cparts, C.int(i))
		setupTopicPartitionFromCrktpar(&partitions[i], crktpar)
	}

	return partitions
}

// LibraryVersion returns the underlying librdkafka library version as a
// (version_int, version_str) tuple.
func LibraryVersion() (int, string) {
	ver := (int)(C.rd_kafka_version())
	verstr := C.GoString(C.rd_kafka_version_str())
	return ver, verstr
}
