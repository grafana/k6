package ping

import (
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/comm"
	"github.com/loadimpact/speedboat/util/testutil"
	"testing"
)

func TestParseNothing(t *testing.T) {
	err := testutil.WithAppContext("", Command, func(c *cli.Context) {
		topic, local, err := Parse(c)
		if err != nil {
			t.Error("Error:", err)
		}
		if topic != comm.MasterTopic {
			t.Error("Default topic not master", topic)
		}
		if local {
			t.Error("Default allows local")
		}
	})
	if err != nil {
		t.Error(err)
	}
}

func TestParseWorker(t *testing.T) {
	err := testutil.WithAppContext("--worker", Command, func(c *cli.Context) {
		topic, _, err := Parse(c)
		if err != nil {
			t.Error(err)
		}
		if topic != comm.WorkerTopic {
			t.Fail()
		}
		if err != nil {
			t.Error(err)
		}
	})
	if err != nil {
		t.Error(err)
	}
}

func TestParseLocal(t *testing.T) {
	err := testutil.WithAppContext("--local", Command, func(c *cli.Context) {
		_, local, err := Parse(c)
		if err != nil {
			t.Error(err)
		}
		if !local {
			t.Fail()
		}
	})
	if err != nil {
		t.Error(err)
	}
}
