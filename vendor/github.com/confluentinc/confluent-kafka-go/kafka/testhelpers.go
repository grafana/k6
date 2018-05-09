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
	"encoding/json"
	"fmt"
	"os"
)

/*
#include <librdkafka/rdkafka.h>
*/
import "C"

var testconf struct {
	Brokers      string
	Topic        string
	GroupID      string
	PerfMsgCount int
	PerfMsgSize  int
	Config       []string
	conf         ConfigMap
}

// testconf_read reads the test suite config file testconf.json which must
// contain at least Brokers and Topic string properties.
// Returns true if the testconf was found and usable, false if no such file, or panics
// if the file format is wrong.
func testconfRead() bool {
	cf, err := os.Open("testconf.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%% testconf.json not found - ignoring test\n")
		return false
	}

	// Default values
	testconf.PerfMsgCount = 2000000
	testconf.PerfMsgSize = 100
	testconf.GroupID = "testgroup"

	jp := json.NewDecoder(cf)
	err = jp.Decode(&testconf)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse testconf: %s", err))
	}

	cf.Close()

	if testconf.Brokers[0] == '$' {
		// Read broker list from environment variable
		testconf.Brokers = os.Getenv(testconf.Brokers[1:])
	}

	if testconf.Brokers == "" || testconf.Topic == "" {
		panic("Missing Brokers or Topic in testconf.json")
	}

	return true
}

// update existing ConfigMap with key=value pairs from testconf.Config
func (cm *ConfigMap) updateFromTestconf() error {
	if testconf.Config == nil {
		return nil
	}

	// Translate "key=value" pairs in Config to ConfigMap
	for _, s := range testconf.Config {
		err := cm.Set(s)
		if err != nil {
			return err
		}
	}

	return nil

}

// Return the number of messages available in all partitions of a topic.
// WARNING: This uses watermark offsets so it will be incorrect for compacted topics.
func getMessageCountInTopic(topic string) (int, error) {

	// Create consumer
	c, err := NewConsumer(&ConfigMap{"bootstrap.servers": testconf.Brokers,
		"group.id": testconf.GroupID})
	if err != nil {
		return 0, err
	}

	// get metadata for the topic to find out number of partitions

	metadata, err := c.GetMetadata(&topic, false, 5*1000)
	if err != nil {
		return 0, err
	}

	t, ok := metadata.Topics[topic]
	if !ok {
		return 0, newError(C.RD_KAFKA_RESP_ERR__UNKNOWN_TOPIC)
	}

	cnt := 0
	for _, p := range t.Partitions {
		low, high, err := c.QueryWatermarkOffsets(topic, p.ID, 5*1000)
		if err != nil {
			continue
		}
		cnt += int(high - low)
	}

	return cnt, nil
}
