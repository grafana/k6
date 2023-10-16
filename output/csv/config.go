package csv

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/guregu/null.v3"

	"github.com/mstoykov/envconfig"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/lib/types"
)

// Config is the config for the csv output
type Config struct {
	// Samples.
	FileName     null.String        `json:"file_name" envconfig:"K6_CSV_FILENAME"`
	SaveInterval types.NullDuration `json:"save_interval" envconfig:"K6_CSV_SAVE_INTERVAL"`
	TimeFormat   null.String        `json:"time_format" envconfig:"K6_CSV_TIME_FORMAT"`
}

// TimeFormat custom enum type
//
//go:generate enumer -type=TimeFormat -transform=snake -trimprefix TimeFormat -output time_format_gen.go
type TimeFormat uint8

// valid defined values for TimeFormat
const (
	TimeFormatUnix TimeFormat = iota
	TimeFormatUnixMilli
	TimeFormatUnixMicro
	TimeFormatUnixNano
	TimeFormatRFC3339
	TimeFormatRFC3339Nano
)

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		FileName:     null.NewString("file.csv", false),
		SaveInterval: types.NewNullDuration(1*time.Second, false),
		TimeFormat:   null.NewString("unix", false),
	}
}

// Apply merges two configs by overwriting properties in the old config
func (c Config) Apply(cfg Config) Config {
	if cfg.FileName.Valid {
		c.FileName = cfg.FileName
	}
	if cfg.SaveInterval.Valid {
		c.SaveInterval = cfg.SaveInterval
	}
	if cfg.TimeFormat.Valid {
		c.TimeFormat = cfg.TimeFormat
	}
	return c
}

// ParseArg takes an arg string and converts it to a config
func ParseArg(arg string, logger logrus.FieldLogger) (Config, error) {
	c := NewConfig()

	if !strings.Contains(arg, "=") {
		c.FileName = null.StringFrom(arg)
		return c, nil
	}

	pairs := strings.Split(arg, ",")
	for _, pair := range pairs {
		r := strings.SplitN(pair, "=", 2)
		if len(r) != 2 {
			return c, fmt.Errorf("couldn't parse %q as argument for csv output", arg)
		}
		switch r[0] {
		case "saveInterval":
			err := c.SaveInterval.UnmarshalText([]byte(r[1]))
			if err != nil {
				return c, err
			}
		case "fileName":
			c.FileName = null.StringFrom(r[1])
		case "timeFormat":
			c.TimeFormat = null.StringFrom(r[1])
		default:
			return c, fmt.Errorf("unknown key %q as argument for csv output", r[0])
		}
	}

	return c, nil
}

// GetConsolidatedConfig combines {default config values + JSON config +
// environment vars + arg config values}, and returns the final result.
func GetConsolidatedConfig(
	jsonRawConf json.RawMessage, env map[string]string, arg string, logger logrus.FieldLogger,
) (Config, error) {
	result := NewConfig()
	if jsonRawConf != nil {
		jsonConf := Config{}
		if err := json.Unmarshal(jsonRawConf, &jsonConf); err != nil {
			return result, err
		}
		result = result.Apply(jsonConf)
	}

	envConfig := Config{}
	if err := envconfig.Process("", &envConfig, func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}); err != nil {
		// TODO: get rid of envconfig and actually use the env parameter...
		return result, err
	}
	result = result.Apply(envConfig)

	if arg != "" {
		urlConf, err := ParseArg(arg, logger)
		if err != nil {
			return result, err
		}
		result = result.Apply(urlConf)
	}

	return result, nil
}
