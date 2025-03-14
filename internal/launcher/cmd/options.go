package cmd

import (
	"encoding/json"
	"fmt"

	k6State "go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

// Options contains the optional parameters of the Command function.
type Options struct {
	// BuildServiceURL contains the URL of the k6 build service to be used.
	// If the value is not nil, the k6 binary is built using the build service instead of the local build.
	BuildServiceURL string
	// BuildServiceToken contains the token to be used to authenticate with the build service.
	// Defaults to K6_CLOUD_TOKEN environment variable is set, or the value stored in the k6 config file.
	BuildServiceToken string
}

// CanUseBuildService returns true if the build service can be used.
func (o *Options) CanUseBuildService() bool {
	return o.BuildServiceURL != "" && o.BuildServiceToken != ""
}

// NewOptions creates a new Options object.
func NewOptions(gs *k6State.GlobalState) *Options {
	return &Options{
		BuildServiceURL:   gs.Flags.BuildServiceURL,
		BuildServiceToken: extractToken(gs),
	}
}

func extractToken(gs *k6State.GlobalState) string {
	token, ok := gs.Env["K6_CLOUD_TOKEN"]
	if ok {
		return token
	}

	// load from config file
	config, err := loadConfig(gs)
	if err != nil {
		return ""
	}

	return config.Collectors.Cloud.Token
}

// a simple struct to quickly load the config file
type k6configFile struct {
	Collectors struct {
		Cloud struct {
			Token string `json:"token"`
		} `json:"cloud"`
	} `json:"collectors"`
}

// loadConfig loads the k6 config file from the given path or the default location.
// if using the default location and the file does not exist, it returns an empty config.
func loadConfig(gs *k6State.GlobalState) (k6configFile, error) {
	var (
		config k6configFile
		err    error
	)

	// if not exists, return empty config
	_, err = fsext.Exists(gs.FS, gs.Flags.ConfigFilePath)
	if err != nil {
		return config, nil //nolint:nilerr
	}

	buffer, err := fsext.ReadFile(gs.FS, gs.Flags.ConfigFilePath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file %q: %w", gs.Flags.ConfigFilePath, err)
	}

	err = json.Unmarshal(buffer, &config)
	if err != nil {
		return config, fmt.Errorf("failed to parse config file %q: %w", gs.Flags.ConfigFilePath, err)
	}

	return config, nil
}
