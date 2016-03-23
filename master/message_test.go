package master

import (
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	msg1 := Message{
		Type: "test",
		Body: "Abc123",
	}
	data, err := msg1.Encode()
	if err != nil {
		t.Errorf("Couldn't encode message: %s", err)
	}
	if string(data) != `{"type":"test","body":"Abc123"}` {
		t.Errorf("Incorrect JSON representation: %s", string(data))
	}

	msg2 := &Message{}
	err = DecodeMessage(data, msg2)
	if err != nil {
		t.Errorf("Couldn't decode: %s", err)
	}

	if msg2.Type != msg1.Type {
		t.Errorf("Type mismatch: %s != %s", msg2.Type, msg1.Type)
	}
	if msg2.Body != msg2.Body {
		t.Errorf("Body mismatch: \"%s\" != \"%s\"", msg2.Body, msg1.Body)
	}
}
