package testscript

import (
	"testing"

	"go.k6.io/k6/v2/js/modules"
)

type executionRoot struct {
	tb testing.TB
}

func newExecutionRoot(tb testing.TB) modules.Module {
	tb.Helper()

	return &executionRoot{tb: tb}
}

func (root *executionRoot) NewModuleInstance(_ modules.VU) modules.Instance {
	root.tb.Helper()

	return &executionModule{t: root.tb}
}

type executionModule struct {
	t testing.TB
}

func (m *executionModule) Exports() modules.Exports {
	m.t.Helper()

	return modules.Exports{
		Named: map[string]any{
			"test": map[string]any{
				"fail": func(message string) {
					m.t.Helper()
					m.t.Error(message)
				},
				"abort": func(message string) {
					m.t.Helper()
					m.t.Fatal(message)
				},
			},
		},
	}
}
