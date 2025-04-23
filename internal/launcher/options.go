package launcher

import (
	"encoding/json"
	"fmt"

	k6State "go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

// extractToken gets the cloud token required to access the build service
// from the environment or from the config file
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

// a simplified struct to get the cloud token from the config file, the rest is ignored
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
