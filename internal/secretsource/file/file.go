package file

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"go.k6.io/k6/secretsource"
)

func init() {
	secretsource.RegisterExtension("file", func(params secretsource.Params) (secretsource.SecretSource, error) {
		f, err := params.FS.Open(params.ConfigArgument)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)

		r := make(map[string]string)
		for scanner.Scan() {
			line := scanner.Text()
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				return nil, fmt.Errorf("parsing %q, needs =", line)
			}

			r[k] = v
		}
		return &fileSecretSource{
			internal: r,
		}, nil
	})
}

type fileSecretSource struct {
	internal map[string]string
	filename string
}

func (mss *fileSecretSource) Name() string {
	return "file" // TODO(@mstoykov): make this configurable
}

func (mss *fileSecretSource) Description() string {
	return fmt.Sprintf("file source from %s", mss.filename)
}

func (mss *fileSecretSource) Get(key string) (string, error) {
	v, ok := mss.internal[key]
	if !ok {
		return "", errors.New("no value")
	}
	return v, nil
}
