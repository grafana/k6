package master

import (
	"encoding/json"
)

type Message struct {
	Type string `json:"type"`
	Body string `json:"body"`
}

func DecodeMessage(data []byte) (msg Message, err error) {
	err = json.Unmarshal(data, msg)
	return msg, err
}

func (msg *Message) Encode() ([]byte, error) {
	return json.Marshal(msg)
}
