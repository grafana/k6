/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package loader

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

type loaderFunc func(path string, parts []string) (string, error)

var (
	loaders = []struct {
		name string
		fn   loaderFunc
		expr *regexp.Regexp
	}{
		{"cdnjs", cdnjs, regexp.MustCompile(`^cdnjs.com/libraries/([^/]+)(?:/([(\d\.)]+-?[^/]*))?(?:/(.*))?$`)},
		{"github", github, regexp.MustCompile(`^github.com/([^/]+)/([^/]+)/(.*)$`)},
	}
	invalidScriptErrMsg = `The file "%[1]s" couldn't be found on local disk, ` +
		`and trying to retrieve it from https://%[1]s failed as well. Make ` +
		`sure that you've specified the right path to the file. If you're ` +
		`running k6 using the Docker image make sure you have mounted the ` +
		`local directory (-v /local/path/:/inside/docker/path) containing ` +
		`your script and modules so that they're accessible by k6 from ` +
		`inside of the container, see ` +
		`https://docs.k6.io/v1.0/docs/modules#section-using-local-modules-with-docker.`
)

// Resolves a relative path to an absolute one.
func Resolve(pwd, name string) string {
	if name[0] == '.' {
		return filepath.ToSlash(filepath.Join(pwd, name))
	}
	return name
}

// Returns the directory for the path.
func Dir(name string) string {
	if name == "-" {
		return "/"
	}
	return filepath.Dir(name)
}

func Load(fs afero.Fs, pwd, name string) (*lib.SourceData, error) {
	log.WithFields(log.Fields{"pwd": pwd, "name": name}).Debug("Loading...")

	// We just need to make sure `import ""` doesn't crash the loader.
	if name == "" {
		return nil, errors.New("local or remote path required")
	}

	// Do not allow the protocol to be specified, it messes everything up.
	if strings.Contains(name, "://") {
		return nil, errors.New("imports should not contain a protocol")
	}

	// Do not allow remote-loaded scripts to lift arbitrary files off the user's machine.
	if (name[0] == '/' && pwd[0] != '/') || (filepath.VolumeName(name) != "" && filepath.VolumeName(pwd) == "") {
		return nil, errors.Errorf("origin (%s) not allowed to load local file: %s", pwd, name)
	}

	// If the file starts with ".", resolve it as a relative path.
	name = Resolve(pwd, name)
	log.WithField("name", name).Debug("Resolved...")

	// If the resolved path starts with a "/" or has a volume, it's a local file.
	if name[0] == '/' || filepath.VolumeName(name) != "" {
		data, err := afero.ReadFile(fs, name)
		if err != nil {
			return nil, err
		}
		return &lib.SourceData{Filename: name, Data: data}, nil
	}

	// If the file is from a known service, try loading from there.
	loaderName, loader, loaderArgs := pickLoader(name)
	if loader != nil {
		u, err := loader(name, loaderArgs)
		if err != nil {
			return nil, err
		}
		data, err := fetch(u)
		if err != nil {
			return nil, errors.Wrap(err, loaderName)
		}
		return &lib.SourceData{Filename: name, Data: data}, nil
	}

	// If it's not a file, check is it a remote location. HTTPS is enforced, because it's 2017, HTTPS is easy,
	// running arbitrary, trivially MitM'd code (even sandboxed) is very, very bad.
	origURL := "https://" + name
	parsedURL, err := url.Parse(origURL)

	if err != nil {
		return nil, errors.Errorf(invalidScriptErrMsg, name)
	}

	if _, err = net.LookupHost(parsedURL.Hostname()); err != nil {
		return nil, errors.Errorf(invalidScriptErrMsg, name)
	}

	// Load it and have a look.
	url := origURL
	if !strings.ContainsRune(url, '?') {
		url += "?"
	} else {
		url += "&"
	}
	url += "_k6=1"
	data, err := fetch(url)

	// If this fails, try to fetch without ?_k6=1 - some sources act weird around unknown GET args.
	if err != nil {
		data2, err2 := fetch(origURL)
		if err2 != nil {
			return nil, errors.Errorf(invalidScriptErrMsg, name)
		}
		data = data2
	}

	// TODO: Parse the HTML, look for meta tags!!
	// <meta name="k6-import" content="example.com/path/to/real/file.txt" />
	// <meta name="k6-import" content="github.com/myusername/repo/file.txt" />

	return &lib.SourceData{Filename: name, Data: data}, nil
}

func pickLoader(path string) (string, loaderFunc, []string) {
	for _, loader := range loaders {
		matches := loader.expr.FindAllStringSubmatch(path, -1)
		if len(matches) > 0 {
			return loader.name, loader.fn, matches[0][1:]
		}
	}
	return "", nil, nil
}

func fetch(u string) ([]byte, error) {
	log.WithField("url", u).Debug("Fetching source...")
	startTime := time.Now()
	res, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != 200 {
		switch res.StatusCode {
		case 404:
			return nil, errors.Errorf("not found: %s", u)
		default:
			return nil, errors.Errorf("wrong status code (%d) for: %s", res.StatusCode, u)
		}
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"url": u,
		"t":   time.Since(startTime),
		"len": len(data),
	}).Debug("Fetched!")
	return data, nil
}
