package network

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
)

// Test_rules checks that the queue returns the correct rules for a given config and disruption.
func Test_DisruptorRules(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		filter   Filter
		expected []iptables.Rule
	}{
		{
			name: "both protocol and port specified",
			filter: Filter{
				Port:     6666,
				Protocol: "tcp",
			},
			expected: []iptables.Rule{
				{
					Table: "filter", Chain: "INPUT",
					Args: "-p tcp --dport 6666 -j DROP",
				},
			},
		},
		{
			name: "only protocol specified",
			filter: Filter{
				Protocol: "tcp",
			},
			expected: []iptables.Rule{
				{
					Table: "filter", Chain: "INPUT",
					Args: "-p tcp -j DROP",
				},
			},
		},
		{
			name: "only port specified",
			filter: Filter{
				Port: 8080,
			},
			expected: []iptables.Rule{
				{
					Table: "filter", Chain: "INPUT",
					Args: "--dport 8080 -j DROP",
				},
			},
		},
		{
			name:   "neither protocol nor port specified",
			filter: Filter{},
			expected: []iptables.Rule{
				{
					Table: "filter", Chain: "INPUT",
					Args: "-j DROP",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := Disruptor{
				Filter: tc.filter,
			}

			actual := d.rules()
			if diff := cmp.Diff(actual, tc.expected); diff != "" {
				t.Fatalf("Generated rules do not match expected:\n%s", diff)
			}
		})
	}
}
