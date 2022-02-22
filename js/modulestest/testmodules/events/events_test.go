package events

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/eventloop"
	"go.k6.io/k6/js/modulestest"
)

func TestSetTimeout(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	vu := &modulestest.VU{
		RuntimeField: rt,
		InitEnvField: &common.InitEnvironment{},
		CtxField:     context.Background(),
		StateField:   nil,
	}

	m, ok := New().NewModuleInstance(vu).(*Events)
	require.True(t, ok)
	var log []string
	require.NoError(t, rt.Set("events", m.Exports().Named))
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))
	loop := eventloop.New(vu)
	vu.RegisterCallbackField = loop.RegisterCallback

	err := loop.Start(func() error {
		_, err := vu.Runtime().RunString(`
      events.setTimeout(()=> {
        print("in setTimeout")
      })
      print("outside setTimeout")
      `)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, []string{"outside setTimeout", "in setTimeout"}, log)
}
