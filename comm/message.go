package comm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const ClientTopic string = "client" // Subscription topic for clients
const MasterTopic string = "master" // Subscription topic for masters
const WorkerTopic string = "worker" // Subscription topic for workers

// A directed comm.
type Message struct {
	Topic   string `json:"-"`
	Type    string `json:"type"`
	Payload []byte `json:"payload,omitempty"`
}

func (msg Message) WithPayload(src interface{}) (Message, error) {
	payload, err := json.Marshal(src)
	if err != nil {
		return msg, err
	}
	msg.Payload = payload
	return msg, nil
}

func (msg Message) With(src interface{}) Message {
	msg, err := msg.WithPayload(src)
	if err != nil {
		panic(err)
	}
	return msg
}

func (msg Message) WithError(err error) Message {
	msg.Payload, _ = json.Marshal(err.Error())
	return msg
}

func (msg Message) Take(dst interface{}) error {
	return json.Unmarshal(msg.Payload, dst)
}

func (msg Message) TakeError() error {
	var text string
	if err := msg.Take(&text); err != nil {
		return errors.New(fmt.Sprintf("Failed to decode error: %s", err))
	}
	return errors.New(text)
}

func To(topic, t string) Message {
	return Message{Topic: topic, Type: t}
}

func ToClient(t string) Message {
	return To(ClientTopic, t)
}

func ToMaster(t string) Message {
	return To(MasterTopic, t)
}

func ToWorker(t string) Message {
	return To(WorkerTopic, t)
}

// Decodes a message from wire format.
func Decode(data []byte, msg *Message) (err error) {
	nullIndex := bytes.IndexByte(data, 0)
	msg.Topic = string(data[:nullIndex])
	return json.Unmarshal(data[nullIndex+1:], msg)
}

// Encodes a message to wire format.
func (msg *Message) Encode() ([]byte, error) {
	jdata, err := json.Marshal(msg)
	if err != nil {
		return jdata, err
	}

	buf := bytes.NewBufferString(msg.Topic)
	buf.WriteByte(0)
	buf.Write(jdata)
	return buf.Bytes(), nil
}
