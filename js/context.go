package js

import (
	"fmt"
	"github.com/loadimpact/speedboat"
	"gopkg.in/olebedev/go-duktape.v2"
)

const (
	scriptProp = "__script__"
)

type JSError struct {
	Message  string
	Filename string
	Line     int
}

func (e JSError) Error() string {
	return fmt.Sprintf("%s:%d: %s", e.Filename, e.Line, e.Message)
}

func getJSError(js *duktape.Context) JSError {
	js.GetPropString(-1, "fileName")
	filename := js.SafeToString(-1)
	js.Pop()

	js.GetPropString(-1, "lineNumber")
	line := js.ToInt(-1)
	js.Pop()

	msg := js.SafeToString(-1)
	return JSError{Message: msg, Filename: filename, Line: line}
}

func setupGlobalObject(js *duktape.Context, t speedboat.Test, id int) {
	js.PushGlobalObject()
	defer js.Pop()

	js.PushObject()
	js.PutPropString(-2, "__modules__")

	js.PushObject()
	{
		js.PushInt(id)
		js.PutPropString(-2, "id")

		pushObject(js, t, "")
		js.PutPropString(-2, "test")
	}
	js.PutPropString(-2, "__data__")
}

func putScript(js *duktape.Context, filename, src string) error {
	js.PushGlobalObject()
	defer js.Pop()

	js.PushString(filename)
	if err := js.PcompileStringFilename(0, src); err != nil {
		return err
	}
	js.PutPropString(-2, scriptProp)

	return nil
}

func loadScript(js *duktape.Context, filename, src string) error {
	js.PushString(filename)
	if err := js.PcompileStringFilename(0, src); err != nil {
		return err
	}

	if js.Pcall(0) != duktape.ErrNone {
		err := getJSError(js)
		js.Pop()
		return err
	}
	js.Pop()
	return nil
}
