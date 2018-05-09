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
)


/*
#include <librdkafka/rdkafka.h>

//Minimum required librdkafka version. This is checked both during
//build-time and runtime.
//Make sure to keep the MIN_RD_KAFKA_VERSION, MIN_VER_ERRSTR and #error
//defines and strings in sync.
//

#define MIN_RD_KAFKA_VERSION 0x0000b0400

#ifdef __APPLE__
#define MIN_VER_ERRSTR "confluent-kafka-go requires librdkafka v0.11.4 or later. Install the latest version of librdkafka from Homebrew by running `brew install librdkafka` or `brew upgrade librdkafka`"
#else
#define MIN_VER_ERRSTR "confluent-kafka-go requires librdkafka v0.11.4 or later. Install the latest version of librdkafka from the Confluent repositories, see http://docs.confluent.io/current/installation.html"
#endif

#if RD_KAFKA_VERSION < MIN_RD_KAFKA_VERSION
#ifdef __APPLE__
#error "confluent-kafka-go requires librdkafka v0.11.4 or later. Install the latest version of librdkafka from Homebrew by running `brew install librdkafka` or `brew upgrade librdkafka`"
#else
#error "confluent-kafka-go requires librdkafka v0.11.4 or later. Install the latest version of librdkafka from the Confluent repositories, see http://docs.confluent.io/current/installation.html"
#endif
#endif
*/
import "C"


func versionCheck () error {
	ver, verstr := LibraryVersion()
	if ver < C.MIN_RD_KAFKA_VERSION {
		return newErrorFromString(ErrNotImplemented,
			fmt.Sprintf("%s: librdkafka version %s (0x%x) detected",
				C.MIN_VER_ERRSTR, verstr, ver))
	}
	return nil
}
