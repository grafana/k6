package ping

import (
	"github.com/loadimpact/speedboat/comm"
	"testing"
	"time"
)

func TestProcessPing(t *testing.T) {
	p := PingProcessor{}
	now := time.Now()
	res := <-p.ProcessPing(PingMessage{Time: now})

	if res.Topic != comm.ClientTopic {
		t.Error("Message not to client:", res.Topic)
	}
	if res.Type != "ping.pong" {
		t.Error("Wrong message type:", res.Type)
	}

	data := PingMessage{}
	if err := res.Take(&data); err != nil {
		t.Fatal("Couldn't decode pong:", err)
	}

	if data.Time.Unix() != now.Unix() {
		t.Error("Wrong timestamp:", data.Time)
	}
}
