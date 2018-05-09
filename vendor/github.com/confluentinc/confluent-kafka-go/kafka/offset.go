/**
 * Copyright 2017 Confluent Inc.
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
	"strconv"
)

/*
#include <stdlib.h>
#include <librdkafka/rdkafka.h>

static int64_t _c_rdkafka_offset_tail(int64_t rel) {
   return RD_KAFKA_OFFSET_TAIL(rel);
}
*/
import "C"

// Offset type (int64) with support for canonical names
type Offset int64

// OffsetBeginning represents the earliest offset (logical)
const OffsetBeginning = Offset(C.RD_KAFKA_OFFSET_BEGINNING)

// OffsetEnd represents the latest offset (logical)
const OffsetEnd = Offset(C.RD_KAFKA_OFFSET_END)

// OffsetInvalid represents an invalid/unspecified offset
const OffsetInvalid = Offset(C.RD_KAFKA_OFFSET_INVALID)

// OffsetStored represents a stored offset
const OffsetStored = Offset(C.RD_KAFKA_OFFSET_STORED)

func (o Offset) String() string {
	switch o {
	case OffsetBeginning:
		return "beginning"
	case OffsetEnd:
		return "end"
	case OffsetInvalid:
		return "unset"
	case OffsetStored:
		return "stored"
	default:
		return fmt.Sprintf("%d", int64(o))
	}
}

// Set offset value, see NewOffset()
func (o *Offset) Set(offset interface{}) error {
	n, err := NewOffset(offset)

	if err == nil {
		*o = n
	}

	return err
}

// NewOffset creates a new Offset using the provided logical string, or an
// absolute int64 offset value.
// Logical offsets: "beginning", "earliest", "end", "latest", "unset", "invalid", "stored"
func NewOffset(offset interface{}) (Offset, error) {

	switch v := offset.(type) {
	case string:
		switch v {
		case "beginning":
			fallthrough
		case "earliest":
			return Offset(OffsetBeginning), nil

		case "end":
			fallthrough
		case "latest":
			return Offset(OffsetEnd), nil

		case "unset":
			fallthrough
		case "invalid":
			return Offset(OffsetInvalid), nil

		case "stored":
			return Offset(OffsetStored), nil

		default:
			off, err := strconv.Atoi(v)
			return Offset(off), err
		}

	case int:
		return Offset((int64)(v)), nil
	case int64:
		return Offset(v), nil
	default:
		return OffsetInvalid, newErrorFromString(ErrInvalidArg,
			fmt.Sprintf("Invalid offset type: %t", v))
	}
}

// OffsetTail returns the logical offset relativeOffset from current end of partition
func OffsetTail(relativeOffset Offset) Offset {
	return Offset(C._c_rdkafka_offset_tail(C.int64_t(relativeOffset)))
}

// offsetsForTimes looks up offsets by timestamp for the given partitions.
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
func offsetsForTimes(H Handle, times []TopicPartition, timeoutMs int) (offsets []TopicPartition, err error) {
	cparts := newCPartsFromTopicPartitions(times)
	defer C.rd_kafka_topic_partition_list_destroy(cparts)
	cerr := C.rd_kafka_offsets_for_times(H.gethandle().rk, cparts, C.int(timeoutMs))
	if cerr != C.RD_KAFKA_RESP_ERR_NO_ERROR {
		return nil, newError(cerr)
	}

	return newTopicPartitionsFromCparts(cparts), nil
}
