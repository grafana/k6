package ageval

import (
	"fmt"
	"strconv"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
)

// evalTool is a tool the agent may call. For phase 1 the executor is a JS mock
// (a function or a static value) supplied by the test author. The agent loop
// dispatches by name and does not care how the result is produced, so MCP- and
// sub-agent-backed executors slot in here later without loop changes.
type evalTool struct {
	name        string
	description string
	inputSchema map[string]any

	// mock is an optional JS callable `(input) => string` or a static value
	// used verbatim as the tool result. nil means "use a default ack".
	mock sobek.Value
}

// parseTools reads a JS array of tool definitions into the registry. Each entry
// is `{ name, description, inputSchema, mock }`.
func (mi *ModuleInstance) parseTools(v sobek.Value, reg *toolRegistry) {
	rt := mi.vu.Runtime()
	if v == nil || common.IsNullish(v) {
		return
	}
	arr := v.ToObject(rt)
	lengthVal := arr.Get("length")
	if lengthVal == nil {
		return
	}
	n := int(lengthVal.ToInteger())
	for i := range n {
		item := arr.Get(strconv.Itoa(i))
		if item == nil || sobek.IsUndefined(item) {
			continue
		}
		obj := item.ToObject(rt)
		t := &evalTool{
			name:        getString(obj, "name", ""),
			description: getString(obj, "description", ""),
			mock:        obj.Get("mock"),
		}
		if t.name == "" {
			common.Throw(rt, fmt.Errorf("tool at index %d is missing a name", i))
		}
		if schema := obj.Get("inputSchema"); schema != nil && !common.IsNullish(schema) {
			if m, ok := schema.Export().(map[string]any); ok {
				t.inputSchema = m
			}
		}
		reg.add(t)
	}
}
