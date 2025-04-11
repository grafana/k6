package state

import "path/filepath"

// GlobalOptions contains global config values that apply for all k6 sub-commands.
type GlobalOptions struct {
	ConfigFilePath   string
	Quiet            bool
	NoColor          bool
	Address          string
	ProfilingEnabled bool
	LogOutput        string
	SecretSource     []string
	LogFormat        string
	Verbose          bool
}

// GetDefaultFlags returns the default global flags.
func GetDefaultGlobalOptions(homeDir string) GlobalOptions {
	return GlobalOptions{
		Address:          "localhost:6565",
		ProfilingEnabled: false,
		ConfigFilePath:   filepath.Join(homeDir, "k6", defaultConfigFileName),
		LogOutput:        "stderr",
	}
}

func consolidateGlobalFlags(defaultFlags GlobalOptions, env map[string]string) GlobalOptions {
	result := defaultFlags

	// TODO: add env vars for the rest of the values (after adjusting
	// rootCmdPersistentFlagSet(), of course)

	if val, ok := env["K6_CONFIG"]; ok {
		result.ConfigFilePath = val
	}
	if val, ok := env["K6_LOG_OUTPUT"]; ok {
		result.LogOutput = val
	}
	if val, ok := env["K6_LOG_FORMAT"]; ok {
		result.LogFormat = val
	}
	if env["K6_NO_COLOR"] != "" {
		result.NoColor = true
	}
	// Support https://no-color.org/, even an empty value should disable the
	// color output from k6.
	if _, ok := env["NO_COLOR"]; ok {
		result.NoColor = true
	}
	if _, ok := env["K6_PROFILING_ENABLED"]; ok {
		result.ProfilingEnabled = true
	}
	return result
}
