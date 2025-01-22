// Package loader is about loading files from either the filesystem or through https requests.
package loader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/lib/fsext"
)

// SourceData wraps a source file; data and filename.
type SourceData struct {
	Data []byte
	URL  *url.URL
	PWD  *url.URL
}

const (
	httpsSchemeCouldntBeLoadedMsg = `The moduleSpecifier "%s" couldn't be retrieved from` +
		` the resolved url "%s". Error : "%s"`
	fileSchemeCouldntBeLoadedMsg = `The moduleSpecifier "%s" couldn't be found on ` +
		`local disk. Make sure that you've specified the right path to the file. If you're ` +
		`running k6 using the Docker image make sure you have mounted the ` +
		`local directory (-v /local/path/:/inside/docker/path) containing ` +
		`your script and modules so that they're accessible by k6 from ` +
		`inside of the container, see ` +
		`https://grafana.com/docs/k6/latest/using-k6/modules/#using-local-modules-with-docker.`
)

type unresolvableURLError string

func (u unresolvableURLError) Error() string {
	// TODO potentially add more things about what k6 supports if users report being confused.
	return fmt.Sprintf(`The moduleSpecifier %q couldn't be recognised as something k6 supports.`, (string)(u))
}

// Resolve a relative path to an absolute one.
func Resolve(pwd *url.URL, moduleSpecifier string) (*url.URL, error) {
	if moduleSpecifier == "" {
		return nil, errors.New("local or remote path required")
	}

	if moduleSpecifier[0] == '.' || moduleSpecifier[0] == '/' || filepath.IsAbs(moduleSpecifier) {
		return resolveFilePath(pwd, moduleSpecifier)
	}

	if strings.Contains(moduleSpecifier, "://") {
		u, err := url.Parse(moduleSpecifier)
		if err != nil {
			return nil, err
		}
		if u.Scheme != "file" && u.Scheme != "https" {
			return nil,
				fmt.Errorf("only supported schemes for imports are file and https, %s has `%s`",
					moduleSpecifier, u.Scheme)
		}
		if u.Scheme == "file" && pwd.Scheme == "https" {
			return nil, fmt.Errorf("origin (%s) not allowed to load local file: %s", pwd, moduleSpecifier)
		}
		return u, err
	}

	if strings.HasPrefix(moduleSpecifier, "cdnjs.com") {
		return nil, fmt.Errorf("cdnjs.com 'special' urls are no longer supported - please use real ones. " +
			"You can get yours by going to cdnjs and copy-pasting the full url to the actual JavaScript file")
	}
	if strings.HasPrefix(moduleSpecifier, "github.com") {
		return nil, fmt.Errorf("github.com 'special' urls are no longer supported - please use real ones. " +
			"You can get yours by going to github and copy-pasting the full url to the actual raw JavaScript file")
	}
	return nil, unresolvableURLError(moduleSpecifier)
}

func resolveFilePath(pwd *url.URL, moduleSpecifier string) (*url.URL, error) {
	if pwd.Opaque != "" { // this is a loader reference
		base, dir, _ := strings.Cut(pwd.Opaque, "/")
		if moduleSpecifier[0] == '/' {
			return &url.URL{Opaque: path.Join(base, moduleSpecifier)}, nil
		}
		return &url.URL{Opaque: path.Join(base, path.Join(path.Dir(dir+"/"), moduleSpecifier))}, nil
	}

	// The file is in format like C:/something/path.js. But this will be decoded as scheme `C`
	// ... which is not what we want, we want it to be decoded as file:///C:/something/path.js
	if filepath.VolumeName(moduleSpecifier) != "" {
		moduleSpecifier = "/" + moduleSpecifier
	}

	// we always want for the pwd to end in a slash, but filepath/path.Clean strips it so we read
	// it if it's missing
	finalPwd := pwd
	if pwd.Opaque != "" {
		if !strings.HasSuffix(pwd.Opaque, "/") {
			finalPwd = &url.URL{Opaque: pwd.Opaque + "/"}
		}
	} else if !strings.HasSuffix(pwd.Path, "/") {
		finalPwd = &url.URL{}
		*finalPwd = *pwd
		finalPwd.Path += "/"
	}
	return finalPwd.Parse(moduleSpecifier)
}

// Dir returns the directory for the path.
func Dir(old *url.URL) *url.URL {
	return old.ResolveReference(&url.URL{Path: "./"})
}

// Load loads the provided moduleSpecifier from the given filesystems which are map of fsext.Fs
// for a given scheme which is they key of the map. If the scheme is https then a request will
// be made if the files is not found in the map and written to the map.
func Load(
	logger logrus.FieldLogger, filesystems map[string]fsext.Fs, moduleSpecifier *url.URL, originalModuleSpecifier string,
) (*SourceData, error) {
	logger.WithFields(
		logrus.Fields{
			"moduleSpecifier":         moduleSpecifier,
			"originalModuleSpecifier": originalModuleSpecifier,
		}).Debug("Loading...")

	var pathOnFs string
	switch {
	case moduleSpecifier.Scheme == "":
		pathOnFs = path.Clean(moduleSpecifier.String())
	default:
		pathOnFs = path.Clean(moduleSpecifier.String()[len(moduleSpecifier.Scheme)+len(":/"):])
	}
	scheme := moduleSpecifier.Scheme
	if scheme == "" {
		if moduleSpecifier.Opaque == "" {
			//nolint:stylecheck
			return nil, fmt.Errorf(fileSchemeCouldntBeLoadedMsg, originalModuleSpecifier)
		}
		scheme = "https"
	}

	pathOnFs, err := url.PathUnescape(filepath.FromSlash(pathOnFs))
	if err != nil {
		return nil, err
	}

	data, err := fsext.ReadFile(filesystems[scheme], pathOnFs)

	if err == nil {
		return &SourceData{URL: moduleSpecifier, Data: data}, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if scheme != "https" {
		//nolint:stylecheck
		return nil, fmt.Errorf(fileSchemeCouldntBeLoadedMsg, originalModuleSpecifier)
	}

	finalModuleSpecifierURL := moduleSpecifier

	var result *SourceData
	result, err = loadRemoteURL(logger, finalModuleSpecifierURL)
	if err != nil {
		//nolint:stylecheck
		return nil, fmt.Errorf(httpsSchemeCouldntBeLoadedMsg, originalModuleSpecifier, finalModuleSpecifierURL, err)
	}
	result.URL = moduleSpecifier
	// TODO maybe make an fsext.Fs which makes request directly and than use CacheOnReadFs
	// on top of as with the `file` scheme fs
	_ = fsext.WriteFile(filesystems[scheme], pathOnFs, result.Data, 0o644)

	return result, nil
}

func loadRemoteURL(logger logrus.FieldLogger, u *url.URL) (*SourceData, error) {
	oldQuery := u.RawQuery
	if u.RawQuery != "" {
		u.RawQuery += "&"
	}
	u.RawQuery += "_k6=1"

	data, err := fetch(logger, u.String())

	u.RawQuery = oldQuery
	// If this fails, try to fetch without ?_k6=1 - some sources act weird around unknown GET args.
	if err != nil {
		data, err = fetch(logger, u.String())
		if err != nil {
			return nil, err
		}
	}

	// TODO: Parse the HTML, look for meta tags!!
	// <meta name="k6-import" content="example.com/path/to/real/file.txt" />
	// <meta name="k6-import" content="github.com/myusername/repo/file.txt" />

	return &SourceData{URL: u, Data: data}, nil
}

func fetch(logger logrus.FieldLogger, u string) ([]byte, error) {
	logger.WithField("url", u).Debug("Fetching source...")
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		switch res.StatusCode {
		case http.StatusNotFound:
			return nil, fmt.Errorf("not found: %s", u)
		default:
			return nil, fmt.Errorf("wrong status code (%d) for: %s", res.StatusCode, u)
		}
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	logger.WithFields(logrus.Fields{
		"url": u,
		"t":   time.Since(startTime),
		"len": len(data),
	}).Debug("Fetched!")
	return data, nil
}
