package bridge

import (
	"fmt"
	"strings"
)

type Module struct {
	Name    string
	Members map[string]Func
}

func (m *Module) JS() string {
	jsFuncs := []string{}
	for name, fn := range m.Members {
		jsFuncs = append(jsFuncs, fmt.Sprintf(`'%s': %s`, name, fn.JS(m.Name, name)))
	}
	return fmt.Sprintf("__internal__._register('%s', {\n%s\n});", m.Name, strings.Join(jsFuncs, ",\n"))
}

func BridgeModule(name string, members map[string]interface{}) Module {
	mod := Module{
		Name:    name,
		Members: make(map[string]Func),
	}
	for name, mem := range members {
		mod.Members[name] = BridgeFunc(mem)
	}
	return mod
}
