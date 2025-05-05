package k6provider

import (
	"fmt"
	"regexp"
	"strconv"
)

const (
	sizeIndex = 1
	unitIndex = 2

	kilobytes = 1024
	megabytes = kilobytes * 1024
	gigabytes = megabytes * 1024
)

var (
	errInvalidZizeFormat = fmt.Errorf("invalid size format")

	sizeRe = regexp.MustCompile(`([0-9]+)(Kb|Mb|Gb|)$`)
)

// parseSize parses a size string with a unit suffix and returns the size in bytes.
// supported units are Kb, Mb, Gb. No unit is equivalent to bytes.
func parseSize(sizeString string) (int64, error) {
	if sizeString == "" {
		return 0, nil
	}

	matches := sizeRe.FindStringSubmatch(sizeString)
	if len(matches) == 0 {
		return 0, fmt.Errorf("%w %q", errInvalidZizeFormat, sizeString)
	}

	// convert size to bytes
	multiplier := int64(1)
	switch matches[unitIndex] {
	case "Kb":
		multiplier = kilobytes
	case "Mb":
		multiplier = megabytes
	case "Gb":
		multiplier = gigabytes
	}

	size, err := strconv.ParseInt(matches[sizeIndex], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w %w", errInvalidZizeFormat, err)
	}

	return size * multiplier, nil
}
