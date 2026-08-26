// Package iptables implements objects that manipulate netfilter rules by calling the iptables binary.
package iptables

import (
	"fmt"
	"strings"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

// Iptables adds and removes iptables rules by executing the `iptables` binary.
type Iptables struct {
	// Executor is the runtime.Executor used to run the iptables binary.
	executor runtime.Executor
}

// New returns a new Iptables ready to use.
func New(executor runtime.Executor) Iptables {
	return Iptables{
		executor: executor,
	}
}

// Add appends a rule into the corresponding table and chain.
func (i Iptables) Add(r Rule) error {
	err := i.exec(r.add())
	if err != nil {
		return err
	}

	return nil
}

// Remove removes an existing rule. If the rule does not exist, an error is returned.
func (i Iptables) Remove(r Rule) error {
	err := i.exec(r.remove())
	if err != nil {
		return err
	}

	return nil
}

func (i Iptables) exec(args string) error {
	out, err := i.executor.Exec("iptables", strings.Split(args, " ")...)
	if err != nil {
		return fmt.Errorf("%w: %q", err, out)
	}

	return nil
}

// RuleSet is a stateful object that allows adding rules and keeping track of them to remove them later.
type RuleSet struct {
	iptables Iptables
	rules    []Rule
}

// NewRuleSet builds a RuleSet that uses the provided Iptables instance to add and remove rules.
func NewRuleSet(iptables Iptables) *RuleSet {
	return &RuleSet{
		iptables: iptables,
	}
}

// Add adds a rule. Added rule will be remembered and removed later together with other rules when Remove is called.
func (i *RuleSet) Add(r Rule) error {
	err := i.iptables.Add(r)
	if err != nil {
		return err
	}

	i.rules = append(i.rules, r)

	return nil
}

// Remove removes all added rules. If an error occurs, Remove continues to try and remove remaining rules.
func (i *RuleSet) Remove() error {
	var errors []error

	var remaining []Rule
	for _, rule := range i.rules {
		err := i.iptables.Remove(rule)
		if err != nil {
			errors = append(errors, err)
			remaining = append(remaining, rule)
		}
	}

	i.rules = remaining

	// TODO: Return all errors with errors.Join when we move to go 1.21.
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// Rule is a netfilter/iptables rule.
type Rule struct {
	// Table is the netfilter table to which this rule belongs. It is usually "filter".
	Table string
	// Chain is the netfilter chain to which this rule belongs. Usual values are "INPUT", "OUTPUT".
	Chain string
	// Args is the rest of the netfilter rule.
	// Arguments must be space-separated. Using shell-style quotes or backslashes to group more than one space-separated
	// word as one argument is not allowed.
	Args string
}

func (r Rule) add() string {
	return fmt.Sprintf("-t %s -A %s %s", r.Table, r.Chain, r.Args)
}

func (r Rule) remove() string {
	return fmt.Sprintf("-t %s -D %s %s", r.Table, r.Chain, r.Args)
}
