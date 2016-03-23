package master

import (
	"encoding/json"
)

type Message struct {
	Type string `json:"type"`
	Body string `json:"body"`
}

func DecodeMessage(data []byte, msg interface{}) (err error) {
	return json.Unmarshal(data, msg)
}

func (msg *Message) Encode() ([]byte, error) {
	return json.Marshal(msg)
}
