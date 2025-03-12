package k6build

import (
	"fmt"
	"log/slog"
)

// ParseLogLevel parses the level from a string
func ParseLogLevel(levelString string) (slog.Level, error) {
	var level slog.Level
	err := level.UnmarshalText([]byte(levelString))
	if err != nil {
		return level, fmt.Errorf("parsing log level from string: %w", err)
	}

	return level, nil
}
