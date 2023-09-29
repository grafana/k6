package main

import (
	"testing"
)

func TestChangeFormatTypeNoBody(t *testing.T) {
	t.Parallel()
	c := change{
		Type:   Bug,
		Number: 3231,
		Title:  "Fixes the tracing module sampling option to default to 1.0 when not set by the user.",
		Body:   "",
	}
	exp := "- [#3231](https://github.com/grafana/k6/pull/3231) Fixes the tracing module sampling option to default to 1.0 when not set by the user."
	if s := c.Format(); s != exp {
		t.Errorf("unexpected formatted change: got: %s", s)
	}
}
