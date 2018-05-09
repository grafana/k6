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

// Automatically generate error codes from librdkafka
// See README for instructions
//go:generate $GOPATH/bin/go_rdkafka_generr generated_errors.go

/*
#include <librdkafka/rdkafka.h>
*/
import "C"

// Error provides a Kafka-specific error container
type Error struct {
	code ErrorCode
	str  string
}

func newError(code C.rd_kafka_resp_err_t) (err Error) {
	return Error{ErrorCode(code), ""}
}

func newErrorFromString(code ErrorCode, str string) (err Error) {
	return Error{code, str}
}

func newErrorFromCString(code C.rd_kafka_resp_err_t, cstr *C.char) (err Error) {
	var str string
	if cstr != nil {
		str = C.GoString(cstr)
	} else {
		str = ""
	}
	return Error{ErrorCode(code), str}
}

// Error returns a human readable representation of an Error
// Same as Error.String()
func (e Error) Error() string {
	return e.String()
}

// String returns a human readable representation of an Error
func (e Error) String() string {
	if len(e.str) > 0 {
		return e.str
	}
	return e.code.String()
}

// Code returns the ErrorCode of an Error
func (e Error) Code() ErrorCode {
	return e.code
}
