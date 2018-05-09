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

package kafka

import (
	"fmt"
	"math"
	"time"
	"unsafe"
)

/*
#include <stdlib.h>
#include <librdkafka/rdkafka.h>
#include "glue_rdkafka.h"


#ifdef RD_KAFKA_V_HEADERS
// Convert tmphdrs to chdrs (created by this function).
// If tmphdr.size == -1: value is considered Null
//    tmphdr.size == 0:  value is considered empty (ignored)
//    tmphdr.size > 0:   value is considered non-empty
//
// WARNING: The header values will be freed by this function.
void tmphdrs_to_chdrs (tmphdr_t *tmphdrs, size_t tmphdrsCnt,
                       rd_kafka_headers_t **chdrs) {
   size_t i;

   *chdrs = rd_kafka_headers_new(tmphdrsCnt);

   for (i = 0 ; i < tmphdrsCnt ; i++) {
      rd_kafka_header_add(*chdrs,
                          tmphdrs[i].key, -1,
                          tmphdrs[i].size == -1 ? NULL :
                          (tmphdrs[i].size == 0 ? "" : tmphdrs[i].val),
                          tmphdrs[i].size == -1 ? 0 : tmphdrs[i].size);
      if (tmphdrs[i].size > 0)
         free((void *)tmphdrs[i].val);
   }
}

#else
void free_tmphdrs (tmphdr_t *tmphdrs, size_t tmphdrsCnt) {
   size_t i;
   for (i = 0 ; i < tmphdrsCnt ; i++) {
      if (tmphdrs[i].size > 0)
         free((void *)tmphdrs[i].val);
   }
}
#endif


rd_kafka_resp_err_t do_produce (rd_kafka_t *rk,
          rd_kafka_topic_t *rkt, int32_t partition,
          int msgflags,
          int valIsNull, void *val, size_t val_len,
          int keyIsNull, void *key, size_t key_len,
          int64_t timestamp,
          tmphdr_t *tmphdrs, size_t tmphdrsCnt,
          uintptr_t cgoid) {
  void *valp = valIsNull ? NULL : val;
  void *keyp = keyIsNull ? NULL : key;
#ifdef RD_KAFKA_V_HEADERS
  rd_kafka_headers_t *hdrs = NULL;
#endif


  if (tmphdrsCnt > 0) {
#ifdef RD_KAFKA_V_HEADERS
     tmphdrs_to_chdrs(tmphdrs, tmphdrsCnt, &hdrs);
#else
     free_tmphdrs(tmphdrs, tmphdrsCnt);
     return RD_KAFKA_RESP_ERR__NOT_IMPLEMENTED;
#endif
  }


#ifdef RD_KAFKA_V_TIMESTAMP
  return rd_kafka_producev(rk,
        RD_KAFKA_V_RKT(rkt),
        RD_KAFKA_V_PARTITION(partition),
        RD_KAFKA_V_MSGFLAGS(msgflags),
        RD_KAFKA_V_VALUE(valp, val_len),
        RD_KAFKA_V_KEY(keyp, key_len),
        RD_KAFKA_V_TIMESTAMP(timestamp),
#ifdef RD_KAFKA_V_HEADERS
        RD_KAFKA_V_HEADERS(hdrs),
#endif
        RD_KAFKA_V_OPAQUE((void *)cgoid),
        RD_KAFKA_V_END);
#else
  if (timestamp)
      return RD_KAFKA_RESP_ERR__NOT_IMPLEMENTED;
  if (rd_kafka_produce(rkt, partition, msgflags,
                       valp, val_len,
                       keyp, key_len,
                       (void *)cgoid) == -1)
      return rd_kafka_last_error();
  else
      return RD_KAFKA_RESP_ERR_NO_ERROR;
#endif
}
*/
import "C"

// Producer implements a High-level Apache Kafka Producer instance
type Producer struct {
	events         chan Event
	produceChannel chan *Message
	handle         handle

	// Terminates the poller() goroutine
	pollerTermChan chan bool
}

// String returns a human readable name for a Producer instance
func (p *Producer) String() string {
	return p.handle.String()
}

// get_handle implements the Handle interface
func (p *Producer) gethandle() *handle {
	return &p.handle
}

func (p *Producer) produce(msg *Message, msgFlags int, deliveryChan chan Event) error {
	if msg == nil || msg.TopicPartition.Topic == nil || len(*msg.TopicPartition.Topic) == 0 {
		return newErrorFromString(ErrInvalidArg, "")
	}

	crkt := p.handle.getRkt(*msg.TopicPartition.Topic)

	// Three problems:
	//  1) There's a difference between an empty Value or Key (length 0, proper pointer) and
	//     a null Value or Key (length 0, null pointer).
	//  2) we need to be able to send a null Value or Key, but the unsafe.Pointer(&slice[0])
	//     dereference can't be performed on a nil slice.
	//  3) cgo's pointer checking requires the unsafe.Pointer(slice..) call to be made
	//     in the call to the C function.
	//
	// Solution:
	//  Keep track of whether the Value or Key were nil (1), but let the valp and keyp pointers
	//  point to a 1-byte slice (but the length to send is still 0) so that the dereference (2)
	//  works.
	//  Then perform the unsafe.Pointer() on the valp and keyp pointers (which now either point
	//  to the original msg.Value and msg.Key or to the 1-byte slices) in the call to C (3).
	//
	var valp []byte
	var keyp []byte
	oneByte := []byte{0}
	var valIsNull C.int
	var keyIsNull C.int
	var valLen int
	var keyLen int

	if msg.Value == nil {
		valIsNull = 1
		valLen = 0
		valp = oneByte
	} else {
		valLen = len(msg.Value)
		if valLen > 0 {
			valp = msg.Value
		} else {
			valp = oneByte
		}
	}

	if msg.Key == nil {
		keyIsNull = 1
		keyLen = 0
		keyp = oneByte
	} else {
		keyLen = len(msg.Key)
		if keyLen > 0 {
			keyp = msg.Key
		} else {
			keyp = oneByte
		}
	}

	var cgoid int

	// Per-message state that needs to be retained through the C code:
	//   delivery channel (if specified)
	//   message opaque   (if specified)
	// Since these cant be passed as opaque pointers to the C code,
	// due to cgo constraints, we add them to a per-producer map for lookup
	// when the C code triggers the callbacks or events.
	if deliveryChan != nil || msg.Opaque != nil {
		cgoid = p.handle.cgoPut(cgoDr{deliveryChan: deliveryChan, opaque: msg.Opaque})
	}

	var timestamp int64
	if !msg.Timestamp.IsZero() {
		timestamp = msg.Timestamp.UnixNano() / 1000000
	}

	// Convert headers to C-friendly tmphdrs
	var tmphdrs []C.tmphdr_t
	tmphdrsCnt := len(msg.Headers)

	if tmphdrsCnt > 0 {
		tmphdrs = make([]C.tmphdr_t, tmphdrsCnt)

		for n, hdr := range msg.Headers {
			tmphdrs[n].key = C.CString(hdr.Key)
			if hdr.Value != nil {
				tmphdrs[n].size = C.ssize_t(len(hdr.Value))
				if tmphdrs[n].size > 0 {
					// Make a copy of the value
					// to avoid runtime panic with
					// foreign Go pointers in cgo.
					tmphdrs[n].val = C.CBytes(hdr.Value)
				}
			} else {
				// null value
				tmphdrs[n].size = C.ssize_t(-1)
			}
		}
	} else {
		// no headers, need a dummy tmphdrs of size 1 to avoid index
		// out of bounds panic in do_produce() call below.
		// tmphdrsCnt will be 0.
		tmphdrs = []C.tmphdr_t{{nil, nil, 0}}
	}

	cErr := C.do_produce(p.handle.rk, crkt,
		C.int32_t(msg.TopicPartition.Partition),
		C.int(msgFlags)|C.RD_KAFKA_MSG_F_COPY,
		valIsNull, unsafe.Pointer(&valp[0]), C.size_t(valLen),
		keyIsNull, unsafe.Pointer(&keyp[0]), C.size_t(keyLen),
		C.int64_t(timestamp),
		(*C.tmphdr_t)(unsafe.Pointer(&tmphdrs[0])), C.size_t(tmphdrsCnt),
		(C.uintptr_t)(cgoid))
	if cErr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		if cgoid != 0 {
			p.handle.cgoGet(cgoid)
		}
		return newError(cErr)
	}

	return nil
}

// Produce single message.
// This is an asynchronous call that enqueues the message on the internal
// transmit queue, thus returning immediately.
// The delivery report will be sent on the provided deliveryChan if specified,
// or on the Producer object's Events() channel if not.
// msg.Timestamp requires librdkafka >= 0.9.4 (else returns ErrNotImplemented),
// api.version.request=true, and broker >= 0.10.0.0.
// msg.Headers requires librdkafka >= 0.11.4 (else returns ErrNotImplemented),
// api.version.request=true, and broker >= 0.11.0.0.
// Returns an error if message could not be enqueued.
func (p *Producer) Produce(msg *Message, deliveryChan chan Event) error {
	return p.produce(msg, 0, deliveryChan)
}

// Produce a batch of messages.
// These batches do not relate to the message batches sent to the broker, the latter
// are collected on the fly internally in librdkafka.
// WARNING: This is an experimental API.
// NOTE: timestamps and headers are not supported with this API.
func (p *Producer) produceBatch(topic string, msgs []*Message, msgFlags int) error {
	crkt := p.handle.getRkt(topic)

	cmsgs := make([]C.rd_kafka_message_t, len(msgs))
	for i, m := range msgs {
		p.handle.messageToC(m, &cmsgs[i])
	}
	r := C.rd_kafka_produce_batch(crkt, C.RD_KAFKA_PARTITION_UA, C.int(msgFlags)|C.RD_KAFKA_MSG_F_FREE,
		(*C.rd_kafka_message_t)(&cmsgs[0]), C.int(len(msgs)))
	if r == -1 {
		return newError(C.rd_kafka_last_error())
	}

	return nil
}

// Events returns the Events channel (read)
func (p *Producer) Events() chan Event {
	return p.events
}

// ProduceChannel returns the produce *Message channel (write)
func (p *Producer) ProduceChannel() chan *Message {
	return p.produceChannel
}

// Len returns the number of messages and requests waiting to be transmitted to the broker
// as well as delivery reports queued for the application.
// Includes messages on ProduceChannel.
func (p *Producer) Len() int {
	return len(p.produceChannel) + len(p.events) + int(C.rd_kafka_outq_len(p.handle.rk))
}

// Flush and wait for outstanding messages and requests to complete delivery.
// Includes messages on ProduceChannel.
// Runs until value reaches zero or on timeoutMs.
// Returns the number of outstanding events still un-flushed.
func (p *Producer) Flush(timeoutMs int) int {
	termChan := make(chan bool) // unused stand-in termChan

	d, _ := time.ParseDuration(fmt.Sprintf("%dms", timeoutMs))
	tEnd := time.Now().Add(d)
	for p.Len() > 0 {
		remain := tEnd.Sub(time.Now()).Seconds()
		if remain <= 0.0 {
			return p.Len()
		}

		p.handle.eventPoll(p.events,
			int(math.Min(100, remain*1000)), 1000, termChan)
	}

	return 0
}

// Close a Producer instance.
// The Producer object or its channels are no longer usable after this call.
func (p *Producer) Close() {
	// Wait for poller() (signaled by closing pollerTermChan)
	// and channel_producer() (signaled by closing ProduceChannel)
	close(p.pollerTermChan)
	close(p.produceChannel)
	p.handle.waitTerminated(2)

	close(p.events)

	p.handle.cleanup()

	C.rd_kafka_destroy(p.handle.rk)
}

// NewProducer creates a new high-level Producer instance.
//
// conf is a *ConfigMap with standard librdkafka configuration properties, see here:
//
//
//
//
//
// Supported special configuration properties:
//   go.batch.producer (bool, false) - EXPERIMENTAL: Enable batch producer (for increased performance).
//                                     These batches do not relate to Kafka message batches in any way.
//                                     Note: timestamps and headers are not supported with this interface.
//   go.delivery.reports (bool, true) - Forward per-message delivery reports to the
//                                      Events() channel.
//   go.events.channel.size (int, 1000000) - Events() channel size
//   go.produce.channel.size (int, 1000000) - ProduceChannel() buffer size (in number of messages)
//
func NewProducer(conf *ConfigMap) (*Producer, error) {

	err := versionCheck()
	if err != nil {
		return nil, err
	}

	p := &Producer{}

	v, err := conf.extract("go.batch.producer", false)
	if err != nil {
		return nil, err
	}
	batchProducer := v.(bool)

	v, err = conf.extract("go.delivery.reports", true)
	if err != nil {
		return nil, err
	}
	p.handle.fwdDr = v.(bool)

	v, err = conf.extract("go.events.channel.size", 1000000)
	if err != nil {
		return nil, err
	}
	eventsChanSize := v.(int)

	v, err = conf.extract("go.produce.channel.size", 1000000)
	if err != nil {
		return nil, err
	}
	produceChannelSize := v.(int)

	v, _ = conf.extract("{topic}.produce.offset.report", nil)
	if v == nil {
		// Enable offset reporting by default, unless overriden.
		conf.SetKey("{topic}.produce.offset.report", true)
	}

	// Convert ConfigMap to librdkafka conf_t
	cConf, err := conf.convert()
	if err != nil {
		return nil, err
	}

	cErrstr := (*C.char)(C.malloc(C.size_t(256)))
	defer C.free(unsafe.Pointer(cErrstr))

	C.rd_kafka_conf_set_events(cConf, C.RD_KAFKA_EVENT_DR|C.RD_KAFKA_EVENT_STATS)

	// Create librdkafka producer instance
	p.handle.rk = C.rd_kafka_new(C.RD_KAFKA_PRODUCER, cConf, cErrstr, 256)
	if p.handle.rk == nil {
		return nil, newErrorFromCString(C.RD_KAFKA_RESP_ERR__INVALID_ARG, cErrstr)
	}

	p.handle.p = p
	p.handle.setup()
	p.handle.rkq = C.rd_kafka_queue_get_main(p.handle.rk)
	p.events = make(chan Event, eventsChanSize)
	p.produceChannel = make(chan *Message, produceChannelSize)
	p.pollerTermChan = make(chan bool)

	go poller(p, p.pollerTermChan)

	// non-batch or batch producer, only one must be used
	if batchProducer {
		go channelBatchProducer(p)
	} else {
		go channelProducer(p)
	}

	return p, nil
}

// channel_producer serves the ProduceChannel channel
func channelProducer(p *Producer) {

	for m := range p.produceChannel {
		err := p.produce(m, C.RD_KAFKA_MSG_F_BLOCK, nil)
		if err != nil {
			m.TopicPartition.Error = err
			p.events <- m
		}
	}

	p.handle.terminatedChan <- "channelProducer"
}

// channelBatchProducer serves the ProduceChannel channel and attempts to
// improve cgo performance by using the produceBatch() interface.
func channelBatchProducer(p *Producer) {
	var buffered = make(map[string][]*Message)
	bufferedCnt := 0
	const batchSize int = 1000000
	totMsgCnt := 0
	totBatchCnt := 0

	for m := range p.produceChannel {
		buffered[*m.TopicPartition.Topic] = append(buffered[*m.TopicPartition.Topic], m)
		bufferedCnt++

	loop2:
		for true {
			select {
			case m, ok := <-p.produceChannel:
				if !ok {
					break loop2
				}
				if m == nil {
					panic("nil message received on ProduceChannel")
				}
				if m.TopicPartition.Topic == nil {
					panic(fmt.Sprintf("message without Topic received on ProduceChannel: %v", m))
				}
				buffered[*m.TopicPartition.Topic] = append(buffered[*m.TopicPartition.Topic], m)
				bufferedCnt++
				if bufferedCnt >= batchSize {
					break loop2
				}
			default:
				break loop2
			}
		}

		totBatchCnt++
		totMsgCnt += len(buffered)

		for topic, buffered2 := range buffered {
			err := p.produceBatch(topic, buffered2, C.RD_KAFKA_MSG_F_BLOCK)
			if err != nil {
				for _, m = range buffered2 {
					m.TopicPartition.Error = err
					p.events <- m
				}
			}
		}

		buffered = make(map[string][]*Message)
		bufferedCnt = 0
	}
	p.handle.terminatedChan <- "channelBatchProducer"
}

// poller polls the rd_kafka_t handle for events until signalled for termination
func poller(p *Producer, termChan chan bool) {
out:
	for true {
		select {
		case _ = <-termChan:
			break out

		default:
			_, term := p.handle.eventPoll(p.events, 100, 1000, termChan)
			if term {
				break out
			}
			break
		}
	}

	p.handle.terminatedChan <- "poller"

}

// GetMetadata queries broker for cluster and topic metadata.
// If topic is non-nil only information about that topic is returned, else if
// allTopics is false only information about locally used topics is returned,
// else information about all topics is returned.
func (p *Producer) GetMetadata(topic *string, allTopics bool, timeoutMs int) (*Metadata, error) {
	return getMetadata(p, topic, allTopics, timeoutMs)
}

// QueryWatermarkOffsets returns the broker's low and high offsets for the given topic
// and partition.
func (p *Producer) QueryWatermarkOffsets(topic string, partition int32, timeoutMs int) (low, high int64, err error) {
	return queryWatermarkOffsets(p, topic, partition, timeoutMs)
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
func (p *Producer) OffsetsForTimes(times []TopicPartition, timeoutMs int) (offsets []TopicPartition, err error) {
	return offsetsForTimes(p, times, timeoutMs)
}
