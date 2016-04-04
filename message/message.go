package message

import (
	"bytes"
	"encoding/json"
)

const ClientTopic string = "client" // Subscription topic for clients
const MasterTopic string = "master" // Subscription topic for masters
const WorkerTopic string = "worker" // Subscription topic for workers

// A directed message.
type Message struct {
	Topic   string `json:"-"`
	Type    string `json:"type"`
	Fields  Fields `json:"fields,omitempty"`
	Payload []byte `json:"payload,omitempty"`
}

// A set of fields in a message.
type Fields map[string]interface{}

// Creates a message directed to the master server.
func NewToMaster(t string, f Fields) Message {
	return Message{
		Topic:  MasterTopic,
		Type:   t,
		Fields: f,
	}
}

// Creates a message directed to clients.
func NewToClient(t string, f Fields) Message {
	return Message{
		Topic:  ClientTopic,
		Type:   t,
		Fields: f,
	}
}

// Creates a message directed to workers.
func NewToWorker(t string, f Fields) Message {
	return Message{
		Topic:  WorkerTopic,
		Type:   t,
		Fields: f,
	}
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

func (msg Message) Take(dst interface{}) error {
	return json.Unmarshal(msg.Payload, dst)
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
