package message

import (
	"bytes"
	"encoding/json"
)

const ClientTopic string = "client"
const MasterTopic string = "master"
const WorkerTopic string = "worker"

type Message struct {
	Topic string `json:"topic"`
	Type  string `json:"type"`
	Body  string `json:"body"`
}

func NewToMaster(t string, b string) Message {
	return Message{
		Topic: MasterTopic,
		Type:  t,
		Body:  b,
	}
}

func NewToClient(t string, b string) Message {
	return Message{
		Topic: ClientTopic,
		Type:  t,
		Body:  b,
	}
}

func NewToWorker(t string, b string) Message {
	return Message{
		Topic: ClientTopic,
		Type:  t,
		Body:  b,
	}
}

func Decode(data []byte, msg interface{}) (err error) {
	nullIndex := bytes.IndexByte(data, 0)
	return json.Unmarshal(data[nullIndex+1:], msg)
}

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
