package common

import (
	"errors"
	"testing"

	"github.com/grafana/sobek"
)

func TestThrowConvertsGoError(t *testing.T) {
	rt := sobek.New()
	err := errors.New("boom")

	defer func() {
		t.Helper()
		value := recover()
		thrown, ok := value.(*sobek.Object)
		if !ok {
			t.Fatalf("Throw panic type = %T, want *sobek.Object", value)
		}
		if thrown.String() != "GoError: boom" {
			t.Fatalf("exception = %q, want GoError: boom", thrown.String())
		}
	}()

	Throw(rt, err)
}

func TestThrowPreservesException(t *testing.T) {
	rt := sobek.New()
	_, exception := rt.RunString("throw new Error('bad')")

	defer func() {
		if recover() != exception {
			t.Fatal("Throw did not preserve the existing exception")
		}
	}()

	Throw(rt, exception)
}
