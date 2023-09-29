package main

import (
	"testing"
)

func TestPullTypeString(t *testing.T) {
	t.Parallel()
	typ, err := PullTypeString("epic-feature")
	if err != nil {
		t.Fatalf("got unexpected err: %v", err)
	}
	if typ != EpicFeature {
		t.Errorf("got unexpected pull type: %T %v", typ, typ)
	}
}
