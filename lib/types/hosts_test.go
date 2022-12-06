package types

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Below 4 are port combinations of hostname to Host matching, all checks true condition
	SamePortMapping      = "same port mapping exists, and it should map"
	EmptyPortMapping     = "no port given in mapping but hostname exists, it should map"
	NotGivenPortMapping  = "given port mapping does not exist, it should not map"
	DifferentPortMapping = "different port mapping exists, and it should map"
	// General not existing scenario
	NotInHosts = "mapping should not exists in hosts"

	ExactMatch       = "exact match should happen"
	FallBackWildcard = "should fallback to wildcard"
	// This value is for checking the below scenario.
	// Let's say Hosts contains two values:
	// 1. *.foo.io
	// 2. specific.foo.io
	//
	// When not.specific.foo.io is dialed, it passes three steps:
	// 1. It matches *.foo.io, but continues for exact matching
	// 2. It matches specific.foo.io, but continues for exact matching
	// 3. Encounters that not.specific.foo.io does not exist in trie, and fallbacks to the wildcard (step 1)
	FallBackWildcardEvenMatch = "fallback to wildcard, even after exact match"
)

type HostTestCase struct {
	hostname, expVal, desc string
}

func TestHosts(t *testing.T) {
	t.Parallel()

	hosts, err := NewHosts(map[string]Host{
		// IPv4
		"simple.io":              {IP: net.ParseIP("1.2.3.4")},
		"simple.io:80":           {IP: net.ParseIP("1.2.3.4"), Port: 80},
		"simple.io:443":          {IP: net.ParseIP("1.2.3.4"), Port: 8443},
		"only-port.io:443":       {IP: net.ParseIP("5.6.7.8"), Port: 8443},
		"*.wildcard.io":          {IP: net.ParseIP("9.10.11.12")},
		"specific.wildcard.io":   {IP: net.ParseIP("90.100.110.120")},
		"*wildcard-2.io":         {IP: net.ParseIP("13.14.15.16")},
		"specific.wildcard-2.io": {IP: net.ParseIP("130.140.150.160")},

		// IPv6
		"simple-ipv6.io":              {IP: net.ParseIP("aa::bb")},
		"simple-ipv6.io:80":           {IP: net.ParseIP("aa::bb"), Port: 80},
		"simple-ipv6.io:443":          {IP: net.ParseIP("aa::bb"), Port: 8443},
		"only-port-ipv6.io:443":       {IP: net.ParseIP("cc::dd"), Port: 8443},
		"*.wildcard-ipv6.io":          {IP: net.ParseIP("ee::ff")},
		"specific.wildcard-ipv6.io":   {IP: net.ParseIP("ee:11::ff")},
		"*wildcard-2-ipv6.io":         {IP: net.ParseIP("aa::aa")},
		"specific.wildcard-2-ipv6.io": {IP: net.ParseIP("a1::a1")},
	})

	require.NoError(t, err)

	t.Run("ipv4", func(t *testing.T) {
		t.Parallel()

		t.Run("no trie functionality, simple checks", func(t *testing.T) {
			t.Parallel()

			tcs := []HostTestCase{
				{"simple.io", "1.2.3.4:0", EmptyPortMapping},
				{"simple.io:80", "1.2.3.4:80", SamePortMapping},
				{"simple.io:443", "1.2.3.4:8443", DifferentPortMapping},
				{"simple.io:9999", "", NotGivenPortMapping},
				{"only-port.io", "", NotInHosts},
				{"only-port.io:443", "5.6.7.8:8443", DifferentPortMapping},
				{"only-port.io:9999", "", NotGivenPortMapping},
			}
			runTcs(t, hosts, tcs)
		})

		t.Run("trie functionality", func(t *testing.T) {
			t.Parallel()

			t.Run("*.sub usage", func(t *testing.T) {
				t.Parallel()

				tcs := []HostTestCase{
					{"foo.wildcard.io", "9.10.11.12:0", FallBackWildcard},
					{"specific.wildcard.io", "90.100.110.120:0", ExactMatch},
					{"not.specific.wildcard.io", "9.10.11.12:0", FallBackWildcardEvenMatch},
				}
				runTcs(t, hosts, tcs)
			})

			t.Run("*sub usage", func(t *testing.T) {
				t.Parallel()

				tcs := []HostTestCase{
					{"wildcard-2.io", "13.14.15.16:0", ExactMatch},
					{"foo.wildcard-2.io", "13.14.15.16:0", FallBackWildcard},
					{"prefixwildcard-2.io", "13.14.15.16:0", FallBackWildcard},
					{"specific.wildcard-2.io", "130.140.150.160:0", ExactMatch},
					{"not.specific.wildcard-2.io", "13.14.15.16:0", FallBackWildcardEvenMatch},
				}
				runTcs(t, hosts, tcs)
			})
		})
	})

	t.Run("ipv6", func(t *testing.T) {
		t.Parallel()

		t.Run("no trie functionality, simple checks", func(t *testing.T) {
			t.Parallel()

			tcs := []HostTestCase{
				{"simple-ipv6.io", "[aa::bb]:0", EmptyPortMapping},
				{"simple-ipv6.io:80", "[aa::bb]:80", SamePortMapping},
				{"simple-ipv6.io:443", "[aa::bb]:8443", DifferentPortMapping},
				{"simple-ipv6.io:9999", "", NotGivenPortMapping},
				{"only-port-ipv6.io", "", NotInHosts},
				{"only-port-ipv6.io:443", "[cc::dd]:8443", DifferentPortMapping},
				{"only-port-ipv6.io:9999", "", NotGivenPortMapping},
			}
			runTcs(t, hosts, tcs)
		})

		t.Run("trie functionality", func(t *testing.T) {
			t.Parallel()

			t.Run("*.sub usage", func(t *testing.T) {
				t.Parallel()

				tcs := []HostTestCase{
					{"foo.wildcard-ipv6.io", "[ee::ff]:0", FallBackWildcard},
					{"specific.wildcard-ipv6.io", "[ee:11::ff]:0", ExactMatch},
					{"not.specific.wildcard-ipv6.io", "[ee::ff]:0", FallBackWildcardEvenMatch},
				}
				runTcs(t, hosts, tcs)
			})

			t.Run("*sub usage", func(t *testing.T) {
				t.Parallel()

				tcs := []HostTestCase{
					{"wildcard-2-ipv6.io", "[aa::aa]:0", ExactMatch},
					{"foo.wildcard-2-ipv6.io", "[aa::aa]:0", FallBackWildcard},
					{"prefixwildcard-2-ipv6.io", "[aa::aa]:0", FallBackWildcard},
					{"specific.wildcard-2-ipv6.io", "[a1::a1]:0", ExactMatch},
					{"not.specific.wildcard-2-ipv6.io", "[aa::aa]:0", FallBackWildcardEvenMatch},
				}
				runTcs(t, hosts, tcs)
			})
		})
	})

	t.Run("edge cases", func(t *testing.T) {
		t.Parallel()

		tcs := []HostTestCase{
			{"does-not-exists", "", "simple not existing"},
			{"veeeeeeeeeeeery.veeeeeeeery.loooooooooooooooooooooooooooooooooooong-does-not-exist", "", "long hostname to check end of trie"},
			{"", "", "empty hostname should return empty host"},
			{"*", "", "no match should happen"},
			{"*********************", "", "no match should happen"},
		}
		runTcs(t, hosts, tcs)
	})
}

// runTcs is utility function for testing HostTestCase slice
func runTcs(t *testing.T, at *Hosts, tcs []HostTestCase) {
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.desc+"-"+tc.hostname, func(t *testing.T) {
			t.Parallel()
			addr := at.Match(tc.hostname)

			if tc.expVal != "" {
				require.NotNil(t, addr)
				require.Equal(t, tc.expVal, addr.String())
			} else {
				require.Nil(t, addr)
			}
		})
	}
}

func TestHostsJSON(t *testing.T) {
	t.Parallel()

	hosts, err := NewNullHosts(map[string]Host{
		"example.com":           {IP: net.ParseIP("1.2.3.4"), Port: 0},
		"example-port.com":      {IP: net.ParseIP("5.6.7.8"), Port: 443},
		"example-ipv6.com":      {IP: net.ParseIP("aa::bb"), Port: 0},
		"example-port-ipv6.com": {IP: net.ParseIP("cc::dd"), Port: 443},
	})
	require.NoError(t, err)

	tcs := []struct {
		t       NullHosts
		marshal string
	}{
		{t: NullHosts{}, marshal: nullJSON},
		{
			t: hosts, marshal: `
{
	"example.com":"1.2.3.4",
	"example-port.com":"5.6.7.8:443",
	"example-ipv6.com":"aa::bb",
	"example-port-ipv6.com":"[cc::dd]:443"
}`,
		},
	}

	t.Run("Marshall", func(t *testing.T) {
		t.Parallel()

		for _, tc := range tcs {
			tc := tc
			t.Run(tc.marshal, func(t *testing.T) {
				t.Parallel()
				m, err := json.Marshal(tc.t)
				require.NoError(t, err)
				assert.JSONEq(t, tc.marshal, string(m))
			})
		}
	})

	t.Run("Unmarshall", func(t *testing.T) {
		t.Parallel()

		for _, tc := range tcs {
			tc := tc

			t.Run(tc.marshal, func(t *testing.T) {
				t.Parallel()
				var trie NullHosts
				err := json.Unmarshal([]byte(tc.marshal), &trie)
				require.NoError(t, err)
				assert.Equal(t, tc.t, trie)
			})
		}
	})
}
