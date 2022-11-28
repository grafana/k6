package types

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressTrie(t *testing.T) {
	t.Parallel()

	at := NewAddressTrie(map[string]HostAddress{
		// IPv4
		"simple.io":              {IP: net.ParseIP("1.2.3.4")},
		"simple.io:443":          {IP: net.ParseIP("1.2.3.4"), Port: 8443},
		"with-port.io:80":        {IP: net.ParseIP("5.6.7.8"), Port: 80},
		"with-port.io:443":       {IP: net.ParseIP("5.6.7.8"), Port: 8443},
		"*.wildcard.io":          {IP: net.ParseIP("9.10.11.12")},
		"specific.wildcard.io":   {IP: net.ParseIP("90.100.110.120")},
		"*wildcard-2.io":         {IP: net.ParseIP("13.14.15.16")},
		"specific.wildcard-2.io": {IP: net.ParseIP("130.140.150.160")},

		// IPv6
		"simple-ipv6.io":              {IP: net.ParseIP("aa::bb")},
		"simple-ipv6.io:443":          {IP: net.ParseIP("aa::bb"), Port: 8443},
		"with-port-ipv6.io:80":        {IP: net.ParseIP("cc::dd"), Port: 80},
		"with-port-ipv6.io:443":       {IP: net.ParseIP("cc::dd"), Port: 8443},
		"*.wildcard-ipv6.io":          {IP: net.ParseIP("ee::ff")},
		"specific.wildcard-ipv6.io":   {IP: net.ParseIP("ee:11::ff")},
		"*wildcard-2-ipv6.io":         {IP: net.ParseIP("aa::aa")},
		"specific.wildcard-2-ipv6.io": {IP: net.ParseIP("a1::a1")},
	})

	tcs := []struct {
		hostname, expVal string
	}{
		// IPv4
		{"simple.io", "1.2.3.4:0"},
		{"simple.io:443", "1.2.3.4:8443"},
		{"with-port.io:80", "5.6.7.8:80"},
		{"with-port.io:443", "5.6.7.8:8443"},
		{"with-port.io:800", ""},
		{"foo.wildcard.io", "9.10.11.12:0"},
		{"specific.wildcard.io", "90.100.110.120:0"},
		{"not.specific.wildcard.io", "9.10.11.12:0"},
		{"wildcard-2.io", "13.14.15.16:0"},
		{"prefixwildcard-2.io", "13.14.15.16:0"},
		{"specific.wildcard-2.io", "130.140.150.160:0"},
		{"not.specific.wildcard-2.io", "13.14.15.16:0"},

		// IPv6
		{"simple-ipv6.io", "[aa::bb]:0"},
		{"simple-ipv6.io:443", "[aa::bb]:8443"},
		{"with-port-ipv6.io:80", "[cc::dd]:80"},
		{"with-port-ipv6.io:443", "[cc::dd]:8443"},
		{"with-port-ipv6.io:800", ""},
		{"foo.wildcard-ipv6.io", "[ee::ff]:0"},
		{"specific.wildcard-ipv6.io", "[ee:11::ff]:0"},
		{"wildcard-2-ipv6.io", "[aa::aa]:0"},
		{"foo.wildcard-2-ipv6.io", "[aa::aa]:0"},
		{"prefixwildcard-2-ipv6.io", "[aa::aa]:0"},
		{"specific.wildcard-2-ipv6.io", "[a1::a1]:0"},
		{"not.specific.wildcard-2-ipv6.io", "[aa::aa]:0"},

		// Edge Cases
		{"does-not-exists", ""},
		{"veeeeeeeeeeeery.veeeeeeeery.loooooooooooooooooooooooooooooooooooong-does-not-exist", ""},
		{"", ""},
		{"*", ""},
		{"*********************", ""},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.hostname, func(t *testing.T) {
			t.Parallel()
			addr := at.Match(tc.hostname)

			if tc.expVal != "" {
				require.NotNil(t, addr)
				require.Equal(t, tc.expVal, addr.String())
			}
		})
	}
}

func TestNullAddressTrieJSON(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		t       NullAddressTrie
		marshal string
	}{
		{t: NullAddressTrie{}, marshal: "null"},
		{
			t: NewNullAddressTrie(map[string]HostAddress{
				"example.com":           {IP: net.ParseIP("1.2.3.4"), Port: 0},
				"example-port.com":      {IP: net.ParseIP("5.6.7.8"), Port: 443},
				"example-ipv6.com":      {IP: net.ParseIP("aa::bb"), Port: 0},
				"example-port-ipv6.com": {IP: net.ParseIP("cc::dd"), Port: 443},
			}),
			marshal: `
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
				var trie NullAddressTrie
				err := json.Unmarshal([]byte(tc.marshal), &trie)
				require.NoError(t, err)
				assert.Equal(t, tc.t, trie)
			})
		}
	})
}
