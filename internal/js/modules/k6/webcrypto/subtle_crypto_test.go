package webcrypto_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

func TestGenerateKeyPrivateKeyUsages(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		algorithm         string
		extractable       bool
		usages            []string
		wantStatus        string
		wantErrorName     string
		wantPrivateUsages []string
		wantPublicUsages  []string
	}{
		{
			name:          "ECDSA verify only extractable",
			algorithm:     `{"name":"ECDSA","namedCurve":"P-256"}`,
			extractable:   true,
			usages:        []string{"verify"},
			wantStatus:    "rejected",
			wantErrorName: "SyntaxError",
		},
		{
			name:          "ECDSA verify only not extractable",
			algorithm:     `{"name":"ECDSA","namedCurve":"P-256"}`,
			extractable:   false,
			usages:        []string{"verify"},
			wantStatus:    "rejected",
			wantErrorName: "SyntaxError",
		},
		{
			name:          "RSA-PSS verify only",
			algorithm:     `{"name":"RSA-PSS","modulusLength":2048,"publicExponent":new Uint8Array([1,0,1]),"hash":"SHA-256"}`,
			extractable:   true,
			usages:        []string{"verify"},
			wantStatus:    "rejected",
			wantErrorName: "SyntaxError",
		},
		{
			name:          "RSASSA-PKCS1-v1_5 verify only",
			algorithm:     `{"name":"RSASSA-PKCS1-v1_5","modulusLength":2048,"publicExponent":new Uint8Array([1,0,1]),"hash":"SHA-256"}`,
			extractable:   true,
			usages:        []string{"verify"},
			wantStatus:    "rejected",
			wantErrorName: "SyntaxError",
		},
		{
			name:          "RSA-OAEP encrypt only",
			algorithm:     `{"name":"RSA-OAEP","modulusLength":2048,"publicExponent":new Uint8Array([1,0,1]),"hash":"SHA-256"}`,
			extractable:   true,
			usages:        []string{"encrypt"},
			wantStatus:    "rejected",
			wantErrorName: "SyntaxError",
		},
		{
			name:          "RSA-OAEP wrapKey only",
			algorithm:     `{"name":"RSA-OAEP","modulusLength":2048,"publicExponent":new Uint8Array([1,0,1]),"hash":"SHA-256"}`,
			extractable:   true,
			usages:        []string{"wrapKey"},
			wantStatus:    "rejected",
			wantErrorName: "SyntaxError",
		},
		{
			name:          "RSA-OAEP encrypt and wrapKey",
			algorithm:     `{"name":"RSA-OAEP","modulusLength":2048,"publicExponent":new Uint8Array([1,0,1]),"hash":"SHA-256"}`,
			extractable:   true,
			usages:        []string{"encrypt", "wrapKey"},
			wantStatus:    "rejected",
			wantErrorName: "SyntaxError",
		},
		{
			name:              "ECDSA sign only",
			algorithm:         `{"name":"ECDSA","namedCurve":"P-256"}`,
			extractable:       true,
			usages:            []string{"sign"},
			wantStatus:        "resolved",
			wantPrivateUsages: []string{"sign"},
			wantPublicUsages:  []string{},
		},
		{
			name:              "ECDSA sign and verify",
			algorithm:         `{"name":"ECDSA","namedCurve":"P-256"}`,
			extractable:       true,
			usages:            []string{"sign", "verify"},
			wantStatus:        "resolved",
			wantPrivateUsages: []string{"sign"},
			wantPublicUsages:  []string{"verify"},
		},
		{
			name:              "RSA-OAEP decrypt only",
			algorithm:         `{"name":"RSA-OAEP","modulusLength":2048,"publicExponent":new Uint8Array([1,0,1]),"hash":"SHA-256"}`,
			extractable:       true,
			usages:            []string{"decrypt"},
			wantStatus:        "resolved",
			wantPrivateUsages: []string{"decrypt"},
			wantPublicUsages:  []string{},
		},
		{
			name:              "ECDH deriveBits only",
			algorithm:         `{"name":"ECDH","namedCurve":"P-256"}`,
			extractable:       true,
			usages:            []string{"deriveBits"},
			wantStatus:        "resolved",
			wantPrivateUsages: []string{"deriveBits"},
			wantPublicUsages:  []string{},
		},
		{
			name:          "ECDSA empty usages",
			algorithm:     `{"name":"ECDSA","namedCurve":"P-256"}`,
			extractable:   true,
			usages:        []string{},
			wantStatus:    "rejected",
			wantErrorName: "SyntaxError",
		},
		{
			name:          "AES-GCM empty usages",
			algorithm:     `{"name":"AES-GCM","length":128}`,
			extractable:   true,
			usages:        []string{},
			wantStatus:    "rejected",
			wantErrorName: "SyntaxError",
		},
		{
			name:              "AES-GCM encrypt",
			algorithm:         `{"name":"AES-GCM","length":128}`,
			extractable:       true,
			usages:            []string{"encrypt"},
			wantStatus:        "resolved",
			wantPrivateUsages: []string{"encrypt"},
			wantPublicUsages:  []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rt := modulestest.NewRuntime(t)

			usages, err := json.Marshal(tc.usages)
			require.NoError(t, err)

			code := fmt.Sprintf(`
				globalThis.outcome = "__pending__";
				crypto.subtle.generateKey(%s, %t, %s).then(
					keys => {
						const privateKey = keys.privateKey || keys;
						const publicKey = keys.publicKey;
						globalThis.outcome = JSON.stringify({
							status: "resolved",
							privateUsages: privateKey.usages,
							publicUsages: publicKey ? publicKey.usages : []
						});
					},
					error => {
						globalThis.outcome = JSON.stringify({
							status: "rejected",
							name: error.name
						});
					}
				);
			`, tc.algorithm, tc.extractable, usages)
			_, err = rt.RunOnEventLoop(code)
			require.NoError(t, err)

			outcomeValue := rt.VU.Runtime().Get("outcome")
			require.NotNil(t, outcomeValue)
			require.NotEqual(t, "__pending__", outcomeValue.String(), "promise callback did not run")

			var outcome struct {
				Status        string   `json:"status"`
				Name          string   `json:"name"`
				PrivateUsages []string `json:"privateUsages"`
				PublicUsages  []string `json:"publicUsages"`
			}
			require.NoError(t, json.Unmarshal([]byte(outcomeValue.String()), &outcome))
			assert.Equal(t, tc.wantStatus, outcome.Status)
			if tc.wantStatus == "rejected" {
				assert.Equal(t, tc.wantErrorName, outcome.Name)
				return
			}

			assert.Equal(t, tc.wantPrivateUsages, outcome.PrivateUsages)
			assert.Equal(t, tc.wantPublicUsages, outcome.PublicUsages)
		})
	}
}
