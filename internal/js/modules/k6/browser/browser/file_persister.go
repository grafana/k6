package browser

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/storage"
)

type presignedURLConfig struct {
	getterURL  string
	headers    map[string]string
	basePath   string
	upload     string // empty means uploadPresign
	formFields map[string]string
}

const (
	uploadPresign = "presign"
	uploadDirect  = "direct"
)

// newScreenshotPersister will return either a persister that persists file to the local
// disk or uploads the files to a remote location. This decision depends on whether
// the K6_BROWSER_SCREENSHOTS_OUTPUT env var is setup with the correct configs.
func newScreenshotPersister(envLookup env.LookupFunc, logger logrus.FieldLogger) (filePersister, error) {
	envVar, ok := envLookup(env.ScreenshotsOutput)
	if !ok || envVar == "" {
		return &storage.LocalFilePersister{}, nil
	}

	popts, err := parsePresignedURLEnvVar(envVar)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", env.ScreenshotsOutput, err)
	}

	if popts.upload == uploadDirect {
		return storage.NewDirectFilePersister(
			popts.getterURL, popts.headers, popts.formFields, popts.basePath, logger,
		), nil
	}

	return storage.NewRemoteFilePersister(popts.getterURL, popts.headers, popts.basePath, logger), nil
}

// parsePresignedURLEnvVar will parse a value such as:
// url=https://127.0.0.1/,basePath=/screenshots,header.1=a,header.2=b
// and return them.
//
// With upload=direct, url is the endpoint that receives the file and its
// metadata in a single request, and field.k=v entries are sent as multipart
// form fields. The default, upload=presign, requests a presigned URL from url
// first and then uploads the file to it.
//

func parsePresignedURLEnvVar(envVarValue string) (presignedURLConfig, error) {
	ss := strings.Split(envVarValue, ",")

	presignedURL := presignedURLConfig{
		headers: make(map[string]string),
	}
	// Errors must not include values, as they can hold credentials
	// such as Authorization header values.
	for i, s := range ss {
		// The key value pair should be of the form key=value, so split
		// on the first '=' to retrieve the key and value separately. The
		// value may itself contain '=', e.g. base64 padding or a query string.
		k, v, ok := strings.Cut(s, "=")
		if !ok || k == "" {
			return presignedURLConfig{}, fmt.Errorf("format of value must be k=v, at entry %d", i+1)
		}

		if fk, ok := strings.CutPrefix(k, "field."); ok {
			if err := presignedURL.addFormField(fk, v, i+1); err != nil {
				return presignedURLConfig{}, err
			}
			continue
		}

		// A key with "header." means that the header name is encoded in the
		// key, separated by a ".". Split the header on "." to retrieve the
		// header name. The header value should be present in v from the previous
		// split.
		var hv, hk string
		if strings.Contains(k, "header.") {
			hv = v

			hh := strings.Split(k, ".")
			if len(hh) != 2 {
				return presignedURLConfig{}, fmt.Errorf("format of header must be header.k=v, received key %q", k)
			}

			k = hh[0]
			hk = hh[1]

			if hk == "" {
				return presignedURLConfig{}, fmt.Errorf("empty header key, at entry %d", i+1)
			}
		}

		switch k {
		case "url":
			// The url must be absolute, since it's used to make requests.
			u, err := url.ParseRequestURI(v)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return presignedURLConfig{}, errors.New("invalid url")
			}
			presignedURL.getterURL = v
		case "basePath":
			presignedURL.basePath = v
		case "upload":
			presignedURL.upload = v
		case "header":
			presignedURL.headers[hk] = hv
		default:
			return presignedURLConfig{}, fmt.Errorf("invalid option %q", k)
		}
	}

	if presignedURL.getterURL == "" {
		return presignedURLConfig{}, errors.New("missing required url")
	}
	if presignedURL.upload != "" && presignedURL.upload != uploadPresign && presignedURL.upload != uploadDirect {
		return presignedURLConfig{}, fmt.Errorf("upload must be %q or %q", uploadPresign, uploadDirect)
	}
	if presignedURL.upload != uploadDirect && len(presignedURL.formFields) > 0 {
		return presignedURLConfig{}, errors.New("field options are only supported with upload=direct")
	}

	return presignedURL, nil
}

// addFormField adds a field.k=v entry, found at the given position of the
// env var, as a multipart form field.
func (c *presignedURLConfig) addFormField(key, value string, entry int) error {
	if key == "" {
		return fmt.Errorf("empty field key, at entry %d", entry)
	}
	if key == storage.CapturedAtField {
		return fmt.Errorf("field %q is reserved, at entry %d", key, entry)
	}
	if c.formFields == nil {
		c.formFields = make(map[string]string)
	}
	c.formFields[key] = value

	return nil
}
