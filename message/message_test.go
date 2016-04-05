package message

import (
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	msg1 := Message{
		Topic:   "test",
		Type:    "test",
		Payload: []byte("test"),
	}
	data, err := msg1.Encode()
	msg2 := &Message{}
	err = Decode(data, msg2)
	if err != nil {
		t.Errorf("Couldn't decode: %s", err)
	}

	if msg2.Topic != msg1.Topic {
		t.Errorf("Topic mismatch: %s != %s", msg2.Topic, msg1.Topic)
	}
	if msg2.Type != msg1.Type {
		t.Errorf("Type mismatch: %s != %s", msg2.Type, msg1.Type)
	}
	if string(msg2.Payload) != string(msg1.Payload) {
		t.Errorf("Payload mismatch: \"%s\" != \"%s\"", string(msg2.Payload), string(msg1.Payload))
	}
}
