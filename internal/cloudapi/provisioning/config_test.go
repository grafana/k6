package provisioning

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib/types"
)

func TestConfig_Apply(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		base    Config
		applied Config
		want    Config
	}{
		{
			name: "merges populated fields",
			base: Config{
				Host: null.StringFrom("https://base.example.com"),
			},
			applied: Config{
				Token:   null.StringFrom("new-token"),
				Timeout: types.NewNullDuration(10*time.Second, true),
			},
			want: Config{
				Token:   null.StringFrom("new-token"),
				Host:    null.StringFrom("https://base.example.com"),
				Timeout: types.NewNullDuration(10*time.Second, true),
			},
		},
		{
			name: "unset fields don't clobber base",
			base: Config{
				Token:   null.StringFrom("original-token"),
				Host:    null.StringFrom("https://base.example.com"),
				Timeout: types.NewNullDuration(5*time.Second, true),
			},
			applied: Config{},
			want: Config{
				Token:   null.StringFrom("original-token"),
				Host:    null.StringFrom("https://base.example.com"),
				Timeout: types.NewNullDuration(5*time.Second, true),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.base.Apply(tc.applied)
			assert.Equal(t, tc.want, got)
		})
	}
}
