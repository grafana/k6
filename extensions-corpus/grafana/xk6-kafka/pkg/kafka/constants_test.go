package kafka

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// contractConstantRE matches the flat `export const NAME: VALUE;` declarations
// in index.d.ts. Only the flat constants use `export const`; type aliases use
// `export type`, so this captures exactly the contract's constant surface.
var contractConstantRE = regexp.MustCompile(`(?m)^export const ([A-Z0-9_]+):\s*(.+?);`)

// parseContractConstants reads the authoritative contract (index.d.ts) and
// returns its flat constants as a name→value map (string or int64). The test
// compares the runtime against this, so drift between the runtime and the
// source of truth is caught — not just drift from a second hand-kept copy.
func parseContractConstants(t *testing.T) map[string]any {
	t.Helper()
	src, err := os.ReadFile("../../index.d.ts") //nolint:forbidigo // test reads the contract file
	if err != nil {
		t.Fatalf("read index.d.ts: %v", err)
	}
	out := map[string]any{}
	for _, match := range contractConstantRE.FindAllStringSubmatch(string(src), -1) {
		name, raw := match[1], strings.TrimSpace(match[2])
		if strings.HasPrefix(raw, `"`) {
			out[name] = strings.Trim(raw, `"`)
			continue
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			t.Fatalf("constant %s: cannot parse value %q: %v", name, raw, err)
		}
		out[name] = n
	}
	if len(out) == 0 {
		t.Fatal("no constants parsed from index.d.ts")
	}
	return out
}

func TestModuleConstantsMatchContract(t *testing.T) {
	t.Parallel()

	want := parseContractConstants(t)
	got := moduleConstants()

	if len(got) != len(want) {
		t.Errorf("constant count: got %d, want %d", len(got), len(want))
	}
	for name, wantVal := range want {
		gotVal, ok := got[name]
		if !ok {
			t.Errorf("missing constant %q (declared in index.d.ts)", name)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("constant %q: got %v (%T), want %v (%T)", name, gotVal, gotVal, wantVal, wantVal)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("constant %q exported but not declared in index.d.ts", name)
		}
	}
}
