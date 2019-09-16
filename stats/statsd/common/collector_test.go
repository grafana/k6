package common

import (
	"testing"

	"github.com/loadimpact/k6/stats"

	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

func TestInitWithoutAddressErrors(t *testing.T) {
	var c = &Collector{
		Config: Config{},
		Type:   "testtype",
	}
	err := c.Init()
	require.Error(t, err)
}

func TestInitWithBogusAddressErrors(t *testing.T) {
	var c = &Collector{
		Config: Config{
			Addr: null.StringFrom("localhost:90000"),
		},
		Type: "testtype",
	}
	err := c.Init()
	require.Error(t, err)
}

func TestLinkReturnAddress(t *testing.T) {
	var bogusValue = "bogus value"
	var c = &Collector{
		Config: Config{
			Addr: null.StringFrom(bogusValue),
		},
	}
	require.Equal(t, bogusValue, c.Link())
}

func TestGetRequiredSystemTags(t *testing.T) {
	var c = &Collector{}
	require.Equal(t, stats.SystemTagSet(0), c.GetRequiredSystemTags())
}
