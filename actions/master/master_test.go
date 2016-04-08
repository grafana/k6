package master

import (
	"testing"
)

func TestMasterAddresses(t *testing.T) {
	in, out := MasterAddresses("127.0.0.1", 1234)
	if in != "tcp://127.0.0.1:1234" {
		t.Error("Invalid in address", in)
	}
	if out != "tcp://127.0.0.1:1235" {
		t.Error("Invalid out address", out)
	}
}
