package extensionapi

import (
	"testing"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	name := "k6/x/extension-api-test-register"
	value := &struct{}{}
	Register(name, value)

	registered := Registered()
	if registered[name] != value {
		t.Fatal("registered module is not preserved")
	}

	registered[name] = nil
	if Registered()[name] != value {
		t.Fatal("Registered returned the mutable registry")
	}
}

func TestRegisterRejectsInvalidName(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("Register did not panic")
		}
	}()

	Register("example", struct{}{})
}
