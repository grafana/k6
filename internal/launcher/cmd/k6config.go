package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// structure of the config file with the fields that are used by k6exec
type k6configFile struct {
	Collectors struct {
		Cloud struct {
			Token string `json:"token"`
		} `json:"cloud"`
	} `json:"collectors"`
}

// loadConfig loads the k6 config file from the given path or the default location.
// if using the default location and the file does not exist, it returns an empty config.
// TODO: migrate to k6's loading
func loadConfig(configPath string) (k6configFile, error) {
	var (
		config       k6configFile
		usingDefault bool
		homeDir      string
		err          error
	)

	if configPath == "" {
		usingDefault = true
		homeDir, err = os.UserConfigDir() //nolint:forbidigo
		if err != nil {
			return config, fmt.Errorf("failed to get user home directory: %w", err)
		}
		// FIXME: it's not only on loadimpact anymore
		configPath = filepath.Join(homeDir, "loadimpact", "k6", "config.json")
	}

	buffer, err := os.ReadFile(configPath) //nolint:forbidigo,gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && usingDefault { //nolint:forbidigo
			return config, nil
		}
		return config, fmt.Errorf("failed to read config file %q: %w", configPath, err)
	}

	err = json.Unmarshal(buffer, &config)
	if err != nil {
		return config, fmt.Errorf("failed to parse config file %q: %w", configPath, err)
	}

	return config, nil
}
