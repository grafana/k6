package iptables

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

func Test_Iptables(t *testing.T) {
	t.Parallel()

	anError := errors.New("an error occurred")

	for _, tc := range []struct {
		name             string
		testFunc         func(Iptables) error
		execError        error
		expectedCommands []string
		expectedError    error
	}{
		{
			name: "Adds rule",
			testFunc: func(i Iptables) error {
				return i.Add(Rule{
					Table: "some",
					Chain: "ECHO",
					Args:  "foo -t bar -w xx",
				})
			},
			expectedCommands: []string{
				"iptables -t some -A ECHO foo -t bar -w xx",
			},
		},
		{
			name: "Removes rule",
			testFunc: func(i Iptables) error {
				return i.Remove(Rule{
					Table: "some",
					Chain: "ECHO",
					Args:  "foo -t bar -w xx",
				})
			},
			expectedCommands: []string{
				"iptables -t some -D ECHO foo -t bar -w xx",
			},
		},
		{
			name: "Propagates error",
			testFunc: func(i Iptables) error {
				return i.Remove(Rule{
					Table: "some",
					Chain: "ECHO",
					Args:  "foo -t bar -w xx",
				})
			},
			execError: anError,
			expectedCommands: []string{
				"iptables -t some -D ECHO foo -t bar -w xx",
			},
			expectedError: anError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fakeExec := runtime.NewFakeExecutor(nil, tc.execError)
			ipt := New(fakeExec)
			err := tc.testFunc(ipt)
			if !errors.Is(err, tc.expectedError) {
				t.Fatalf("Expected error to be %v, got %v", tc.expectedError, err)
			}

			commands := fakeExec.CmdHistory()
			if diff := cmp.Diff(commands, tc.expectedCommands); diff != "" {
				t.Fatalf("Ran commands do not match expected:\n%s", diff)
			}
		})
	}
}

func Test_RulesetAddsRemovesRules(t *testing.T) {
	t.Parallel()

	exec := runtime.NewFakeExecutor(nil, nil)
	ruleset := NewRuleSet(New(exec))

	// Add two rules
	err := ruleset.Add(Rule{
		Table: "table1", Chain: "CHAIN1", Args: "--foo foo --bar bar",
	})
	if err != nil {
		t.Fatalf("error adding rule: %v", err)
	}

	err = ruleset.Add(Rule{
		Table: "table2", Chain: "CHAIN2", Args: "--boo boo --baz baz",
	})
	if err != nil {
		t.Fatalf("error adding rule: %v", err)
	}

	// Check we have run the expected commands.
	expectedAddCmds := []string{
		"iptables -t table1 -A CHAIN1 --foo foo --bar bar",
		"iptables -t table2 -A CHAIN2 --boo boo --baz baz",
	}

	if diff := cmp.Diff(exec.CmdHistory(), expectedAddCmds); diff != "" {
		t.Fatalf("Executed commands to add rules do not match expected:\n%s", diff)
	}

	exec.Reset()

	// Remove the rules.
	err = ruleset.Remove()
	if err != nil {
		t.Fatalf("error removing rules: %v", err)
	}

	// Check we have run the expected commands.
	expectedRemoveCmds := []string{
		"iptables -t table1 -D CHAIN1 --foo foo --bar bar",
		"iptables -t table2 -D CHAIN2 --boo boo --baz baz",
	}

	if diff := cmp.Diff(exec.CmdHistory(), expectedRemoveCmds); diff != "" {
		t.Fatalf("Executed commands to remove rules do not match expected:\n%s", diff)
	}

	exec.Reset()

	// After removing the rules, add a new one.
	err = ruleset.Add(Rule{
		Table: "table3", Chain: "CHAIN3", Args: "--zoo zoo --zap zap",
	})
	if err != nil {
		t.Fatalf("error adding rule: %v", err)
	}

	// Check we run the expected add command.
	expectedAddCmds = []string{
		"iptables -t table3 -A CHAIN3 --zoo zoo --zap zap",
	}
	if diff := cmp.Diff(exec.CmdHistory(), expectedAddCmds); diff != "" {
		t.Fatalf("Executed commands to add rules do not match expected:\n%s", diff)
	}

	exec.Reset()

	// Remove the rule.
	err = ruleset.Remove()
	if err != nil {
		t.Fatalf("error removing rules: %v", err)
	}

	// Check we have run the expected command.
	expectedRemoveCmds = []string{
		"iptables -t table3 -D CHAIN3 --zoo zoo --zap zap",
	}

	if diff := cmp.Diff(exec.CmdHistory(), expectedRemoveCmds); diff != "" {
		t.Fatalf("Executed commands to remove rules do not match expected:\n%s", diff)
	}
}
