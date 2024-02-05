package browser

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/grafana/xk6-browser/env"
	"github.com/grafana/xk6-browser/storage"
)

// newFilePersister will return either a persister that persists file to the local
// disk or uploads the files to a remote location. This decision depends on whether
// the K6_BROWSER_SCREENSHOTS_OUTPUT env var is setup with the correct configs.
func newFilePersister(envLookup env.LookupFunc) (filePersister, error) {
	envVar, ok := envLookup(env.ScreenshotsOutput)
	if !ok || envVar == "" {
		return &storage.LocalFilePersister{}, nil
	}

	u, b, h, err := parseEnvVar(envVar)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", env.ScreenshotsOutput, err)
	}

	return storage.NewRemoteFilePersister(u, h, b), nil
}

// parseEnvVar will parse a value such as:
// url=https://127.0.0.1/,basePath=/screenshots,header.1=a,header.2=b
// and return them.
func parseEnvVar(envVarValue string) (string, string, map[string]string, error) {
	ss := strings.Split(envVarValue, ",")

	var (
		getterURL string
		basePath  string
		headers   = make(map[string]string)
	)
	for _, s := range ss {
		// The key value pair should be of the form key=value, so split
		// on '=' to retrieve the key and value separately.
		kv := strings.Split(s, "=")
		if len(kv) != 2 {
			return "", "", nil, fmt.Errorf("format of value must be k=v, received %q", s)
		}

		k := kv[0]
		v := kv[1]

		// A key with "header." means that the header name is encoded in the
		// key, separated by a ".". Split the header on "." to retrieve the
		// header name. The header value should be present in v from the previous
		// split.
		var hv, hk string
		if strings.Contains(k, "header.") {
			hv = v

			hh := strings.Split(k, ".")
			if len(hh) != 2 {
				return "", "", nil, fmt.Errorf("format of header must be header.k=v, received %q", s)
			}

			k = hh[0]
			hk = hh[1]

			if hk == "" {
				return "", "", nil, fmt.Errorf("empty header key, received %q", s)
			}
		}

		switch k {
		case "url":
			u, err := url.ParseRequestURI(v)
			if err != nil && u.Scheme != "" && u.Host != "" {
				return "", "", nil, fmt.Errorf("invalid url %q", s)
			}
			getterURL = v
		case "basePath":
			basePath = v
		case "header":
			headers[hk] = hv
		default:
			return "", "", nil, fmt.Errorf("invalid option %q", k)
		}
	}

	if getterURL == "" {
		return "", "", nil, errors.New("missing required url")
	}

	return getterURL, basePath, headers, nil
}
