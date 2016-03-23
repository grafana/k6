package message

import (
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	msg1 := Message{
		Topic: "test",
		Type:  "test",
		Body:  "Abc123",
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
	if msg2.Body != msg2.Body {
		t.Errorf("Body mismatch: \"%s\" != \"%s\"", msg2.Body, msg1.Body)
	}
}
