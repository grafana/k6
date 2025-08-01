package browser

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestParseContinueOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		jsOpts       string
		expectedOpts common.ContinueOptions
		expectedErr  bool
	}{
		{
			name: "valid_options",
			jsOpts: `{
				postData: "test data",
				method: "POST",
				url: "https://example.com",
				headers: {
					"Content-Type": "application/json",
				}
			}`,
			expectedOpts: common.ContinueOptions{
				PostData: []byte("test data"),
				Headers: []common.HTTPHeader{
					{
						Name:  "Content-Type",
						Value: "application/json",
					},
				},
				Method: "POST",
				URL:    "https://example.com",
			},
			expectedErr: false,
		},
		{
			name: "buffer_postData",
			jsOpts: `{
				postData: new Uint8Array([116, 101, 115, 116, 32, 100, 97, 116, 97]),
				headers: {
					"Content-Type": "application/json",
				}
			}`,
			expectedOpts: common.ContinueOptions{
				PostData: []byte("test data"),
				Headers: []common.HTTPHeader{
					{
						Name:  "Content-Type",
						Value: "application/json",
					},
				},
			},
			expectedErr: false,
		},
		{
			name: "remove_undefined_headers",
			jsOpts: `{
				headers: {
					"Content-Type": undefined,
				}
			}`,
			expectedOpts: common.ContinueOptions{
				Headers: []common.HTTPHeader{},
			},
			expectedErr: false,
		},
		{
			name: "invalid postData",
			jsOpts: `{
				postData: 12345,
				method: "POST"
			}`,
			expectedOpts: common.ContinueOptions{},
			expectedErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			opts, err := vu.Runtime().RunString(
				fmt.Sprintf(`const opts = %s;
					opts;
				`, tt.jsOpts))
			require.NoError(t, err)

			parsedOpts, err := parseContinueOptions(vu.Context(), opts)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOpts, parsedOpts)
			}
		})
	}
}

func TestParseFulfillOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		jsOpts       string
		expectedOpts common.FulfillOptions
		expectedErr  bool
	}{
		{
			name: "valid_options",
			jsOpts: `{
				body: "test data",
				contentType: "application/json",
				status: 200,
			}`,
			expectedOpts: common.FulfillOptions{
				Body:        []byte("test data"),
				ContentType: "application/json",
				Status:      200,
			},
			expectedErr: false,
		},
		{
			name: "buffer_body",
			jsOpts: `{
				body: new Uint8Array([116, 101, 115, 116, 32, 100, 97, 116, 97]),
				headers: {
					"Content-Type": "application/json",
				}
			}`,
			expectedOpts: common.FulfillOptions{
				Body: []byte("test data"),
				Headers: []common.HTTPHeader{
					{
						Name:  "Content-Type",
						Value: "application/json",
					},
				},
			},
			expectedErr: false,
		},
		{
			name: "remove_undefined_headers",
			jsOpts: `{
				headers: {
					"Content-Type": undefined,
				}
			}`,
			expectedOpts: common.FulfillOptions{
				Headers: []common.HTTPHeader{},
			},
			expectedErr: false,
		},
		{
			name: "unsupported_option",
			jsOpts: `{
				unsupportedOption: "value"
			}`,
			expectedOpts: common.FulfillOptions{},
			expectedErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			opts, err := vu.Runtime().RunString(
				fmt.Sprintf(`const opts = %s;
					opts;
				`, tt.jsOpts))
			require.NoError(t, err)

			parsedOpts, err := parseFulfillOptions(vu.Context(), opts)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOpts, parsedOpts)
			}
		})
	}
}
