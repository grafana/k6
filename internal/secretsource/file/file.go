// Package file implements secret source that reads the secrets from a file as key=value pairs one per line
package file

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.k6.io/k6/v2/secretsource"
)

func init() {
	secretsource.RegisterExtension("file", func(params secretsource.Params) (secretsource.Source, error) {
		fss := &fileSecretSource{}
		err := fss.parseArg(params.ConfigArgument)
		if err != nil {
			return nil, err
		}

		f, err := params.FS.Open(fss.filename)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()

		secrets, err := readKeyValueLines(f)
		if err != nil {
			return nil, fmt.Errorf("reading secret file %s: %w", fss.filename, err)
		}
		fss.internal = secrets
		return fss, nil
	})
}

// readKeyValueLines reads key=value secrets, one per line.
//
// bufio.Scanner stops with ErrTooLong once a line exceeds bufio.MaxScanTokenSize
// (64KiB) and does not return that line or anything after it. The error must be
// returned so startup fails instead of silently dropping the rest of the file.
func readKeyValueLines(r io.Reader) (map[string]string, error) {
	scanner := bufio.NewScanner(r)
	secrets := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("parsing %q, needs =", line)
		}
		secrets[k] = v
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return secrets, nil
}

func (fss *fileSecretSource) parseArg(config string) error {
	list := strings.Split(config, ",")
	if len(list) >= 1 {
		for _, kv := range list {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				fss.filename = kv
				continue
			}
			switch k {
			case "filename":
				fss.filename = v
			default:
				return fmt.Errorf("unknown configuration key for file secret source %q", k)
			}
		}
	}
	return nil
}

type fileSecretSource struct {
	internal map[string]string
	filename string
}

func (fss *fileSecretSource) Description() string {
	return fmt.Sprintf("file source from %s", fss.filename)
}

func (fss *fileSecretSource) Get(key string) (string, error) {
	v, ok := fss.internal[key]
	if !ok {
		return "", errors.New("no value")
	}
	return v, nil
}
