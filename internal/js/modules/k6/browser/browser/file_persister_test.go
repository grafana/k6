package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/internal/js/modules/k6/browser/storage"
)

func Test_newScreenshotPersister(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		envLookup env.LookupFunc
		wantType  filePersister
		wantErr   bool
	}{
		{
			name:      "local_no_env_var",
			envLookup: env.EmptyLookup,
			wantType:  &storage.LocalFilePersister{},
		},
		{
			name: "local_empty_env_var",
			envLookup: env.ConstLookup(
				env.ScreenshotsOutput,
				"",
			),
			wantType: &storage.LocalFilePersister{},
		},
		{
			name: "remote",
			envLookup: env.ConstLookup(
				env.ScreenshotsOutput,
				"url=https://127.0.0.1/,basePath=/screenshots,header.1=a",
			),
			wantType: &storage.RemoteFilePersister{},
		},
		{
			name: "remote_parse_failed",
			envLookup: env.ConstLookup(
				env.ScreenshotsOutput,
				"basePath=/screenshots,header.1=a",
			),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotType, err := newScreenshotPersister(tt.envLookup)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.IsType(t, tt.wantType, gotType)
		})
	}
}

func Test_parsePresignedURLEnvVar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		envVarValue string
		want        presignedURLConfig
		wantErr     string
	}{
		{
			name:        "url_headers_basePath",
			envVarValue: "url=https://127.0.0.1/,basePath=/screenshots,header.1=a,header.2=b",
			want: presignedURLConfig{
				getterURL: "https://127.0.0.1/",
				basePath:  "/screenshots",
				headers: map[string]string{
					"1": "a",
					"2": "b",
				},
			},
		},
		{
			name:        "url_headers",
			envVarValue: "url=https://127.0.0.1/,header.1=a,header.2=b",
			want: presignedURLConfig{
				getterURL: "https://127.0.0.1/",
				headers: map[string]string{
					"1": "a",
					"2": "b",
				},
			},
		},
		{
			name:        "url",
			envVarValue: "url=https://127.0.0.1/",
			want: presignedURLConfig{
				getterURL: "https://127.0.0.1/",
				headers:   map[string]string{},
			},
		},
		{
			name:        "url_basePath",
			envVarValue: "url=https://127.0.0.1/,basePath=/screenshots",
			want: presignedURLConfig{
				getterURL: "https://127.0.0.1/",
				basePath:  "/screenshots",
				headers:   map[string]string{},
			},
		},
		{
			name:        "empty_basePath",
			envVarValue: "url=https://127.0.0.1/,basePath=",
			want: presignedURLConfig{
				getterURL: "https://127.0.0.1/",
				basePath:  "",
				headers:   map[string]string{},
			},
		},
		{
			name:        "empty",
			envVarValue: "",
			wantErr:     `format of value must be k=v, received ""`,
		},
		{
			name:        "missing_url",
			envVarValue: "basePath=/screenshots,header.1=a,header.2=b",
			wantErr:     "missing required url",
		},
		{
			name:        "invalid_option",
			envVarValue: "ulr=https://127.0.0.1/",
			wantErr:     "invalid option",
		},
		{
			name:        "empty_header_key",
			envVarValue: "url=https://127.0.0.1/,header.=a",
			wantErr:     "empty header key",
		},
		{
			name:        "invalid_format",
			envVarValue: "url==https://127.0.0.1/",
			wantErr:     "format of value must be k=v",
		},
		{
			name:        "invalid_header_format",
			envVarValue: "url=https://127.0.0.1/,header..asd=a",
			wantErr:     "format of header must be header.k=v",
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parsePresignedURLEnvVar(tt.envVarValue)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
